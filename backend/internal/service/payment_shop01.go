package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// SHOP01 is an Alipay personal-QR collector that attributes an incoming
// payment to an order purely by its exact amount. Two open orders of the same
// instance must therefore never share a pay amount, and an amount must not be
// handed out again right after its order closed: a payer who scanned the old
// QR late would otherwise be credited to the new order.
const (
	// shop01MaxAmountOffsetCents bounds the per-order cent offset k (0..99),
	// i.e. at most 100 concurrent open orders per base amount and instance.
	shop01MaxAmountOffsetCents = 99
	// shop01ClosedAmountReserve keeps amounts of orders that recently became
	// EXPIRED/CANCELLED/FAILED, or were recently paid, occupied: a late scan of
	// the old QR or an accidental second payment must not match a new order.
	shop01ClosedAmountReserve = 10 * time.Minute
	// shop01NotifyWindow is how long after creation a signed notify may still
	// move a locally EXPIRED/CANCELLED order to PAID (SHOP01 retries notify for
	// ~13h). Beyond it the payment is left for manual handling.
	shop01NotifyWindow = 24 * time.Hour
	// shop01GatewayOrderTTL is how long SHOP01 keeps an order payable; we never
	// cancel at the gateway, so a closed order's amount stays live there until
	// created_at+TTL regardless of our local status.
	shop01GatewayOrderTTL        = 30 * time.Minute
	shop01GatewayTTLSafetyMargin = 5 * time.Minute
	shop01AdvisoryLockPrefix     = "payment:shop01:pay_amount:"
)

// ErrShop01ChannelBusy is returned when all 100 cent offsets of a base amount
// are occupied on the selected instance.
var ErrShop01ChannelBusy = infraerrors.TooManyRequests(
	"PAYMENT_CHANNEL_BUSY",
	"payment channel busy, please retry in a minute",
)

func isShop01Selection(sel *payment.InstanceSelection) bool {
	return sel != nil && strings.EqualFold(strings.TrimSpace(sel.ProviderKey), payment.TypeShop01)
}

func isShop01Order(o *dbent.PaymentOrder) bool {
	if o == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(psStringValue(o.ProviderKey)), payment.TypeShop01) {
		return true
	}
	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return strings.EqualFold(strings.TrimSpace(snapshot.ProviderKey), payment.TypeShop01)
	}
	return false
}

// amountToCents converts a 2-decimal amount to integer cents without float
// accumulation. decimal.NewFromFloat uses the shortest representation, so a
// value read from a DECIMAL(20,2) column converts exactly.
func amountToCents(v float64) int64 {
	return decimal.NewFromFloat(v).Shift(2).Round(0).IntPart()
}

func centsToAmount(c int64) float64 {
	return decimal.New(c, -2).InexactFloat64()
}

func shop01AdvisoryLockKey(instanceID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(shop01AdvisoryLockPrefix + strings.TrimSpace(instanceID)))
	return int64(h.Sum64())
}

// shop01Allocation is the outcome of picking a unique pay amount.
type shop01Allocation struct {
	PayCents    int64
	OffsetCents int64
}

// allocateShop01PayAmount must run inside the order-creation transaction. It
// serializes allocation per provider instance with a transaction-scoped
// advisory lock (released on commit/rollback, after the new PENDING row is
// visible), loads the occupied amounts and returns the smallest free
// base+k cents, k in 0..99. Migration 251's partial unique index is the
// database-level safety net behind this lock.
func allocateShop01PayAmount(ctx context.Context, tx *dbent.Tx, instanceID string, basePayAmount float64, now time.Time) (shop01Allocation, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return shop01Allocation{}, fmt.Errorf("shop01 allocation requires a provider instance")
	}
	baseCents := amountToCents(basePayAmount)
	if baseCents <= 0 {
		return shop01Allocation{}, infraerrors.BadRequest("INVALID_AMOUNT", "invalid payment amount")
	}
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", shop01AdvisoryLockKey(instanceID)); err != nil {
		return shop01Allocation{}, fmt.Errorf("lock shop01 pay amounts: %w", err)
	}
	occupiedAmounts, err := tx.PaymentOrder.Query().
		Where(
			paymentorder.ProviderInstanceIDEQ(instanceID),
			paymentorder.ProviderKeyEQ(payment.TypeShop01),
			paymentorder.PayAmountGTE(centsToAmount(baseCents)),
			paymentorder.PayAmountLTE(centsToAmount(baseCents+shop01MaxAmountOffsetCents)),
			paymentorder.Or(
				paymentorder.StatusEQ(OrderStatusPending),
				paymentorder.PaidAtGTE(now.Add(-shop01ClosedAmountReserve)),
				paymentorder.And(
					paymentorder.StatusIn(OrderStatusExpired, OrderStatusCancelled, OrderStatusFailed),
					paymentorder.Or(
						paymentorder.UpdatedAtGTE(now.Add(-shop01ClosedAmountReserve)),
						paymentorder.CreatedAtGTE(now.Add(-(shop01GatewayOrderTTL+shop01GatewayTTLSafetyMargin))),
					),
				),
			),
		).
		Select(paymentorder.FieldPayAmount).
		Float64s(ctx)
	if err != nil {
		return shop01Allocation{}, fmt.Errorf("load occupied shop01 pay amounts: %w", err)
	}
	occupied := make(map[int64]struct{}, len(occupiedAmounts))
	for _, a := range occupiedAmounts {
		occupied[amountToCents(a)] = struct{}{}
	}
	for k := int64(0); k <= shop01MaxAmountOffsetCents; k++ {
		if _, taken := occupied[baseCents+k]; !taken {
			return shop01Allocation{PayCents: baseCents + k, OffsetCents: k}, nil
		}
	}
	return shop01Allocation{}, ErrShop01ChannelBusy
}

// shop01OffsetCredit is the balance credited for the k-cent offset the payer
// pays on top of the quoted amount, converted at the same recharge
// multiplier as the base (multiplier 1 => exactly k cents).
func shop01OffsetCredit(offsetCents int64, multiplier float64) float64 {
	if offsetCents <= 0 {
		return 0
	}
	return calculateCreditedBalance(centsToAmount(offsetCents), multiplier)
}

// shop01ExactAmountCents returns the provider-reported paid amount in cents,
// preferring the raw decimal string carried in metadata.
func shop01ExactAmountCents(paid float64, metadata map[string]string) (int64, error) {
	if raw := strings.TrimSpace(metadata[payment.MetadataKeyExactAmount]); raw != "" {
		return payment.YuanToFen(raw)
	}
	d := decimal.NewFromFloat(paid)
	if !d.Shift(2).Equal(d.Shift(2).Truncate(0)) {
		return 0, fmt.Errorf("paid amount %s has more than 2 decimals", d.String())
	}
	return d.Shift(2).IntPart(), nil
}

// shop01WithinNotifyWindow reports whether a signed notify for o may still be
// credited automatically.
func shop01WithinNotifyWindow(o *dbent.PaymentOrder, now time.Time) bool {
	return o != nil && now.Sub(o.CreatedAt) <= shop01NotifyWindow
}

// shop01PayableStatus is the toPaid status guard for SHOP01 orders. Inside the
// notify window a validly signed, exact-amount notify recovers a locally
// EXPIRED or CANCELLED order regardless of the generic expiry grace: the
// gateway only notifies for money it actually received for this out_trade_no.
func shop01PayableStatus(o *dbent.PaymentOrder, now time.Time) predicate.PaymentOrder {
	if shop01WithinNotifyWindow(o, now) {
		return paymentorder.StatusIn(OrderStatusPending, OrderStatusCancelled, OrderStatusExpired)
	}
	return paymentorder.StatusEQ(OrderStatusPending)
}

// shop01AlreadyProcessed handles a SHOP01 notify whose order could not be moved
// to PAID. A closed order outside the notify window is not acked: the payment
// is real but can no longer be credited automatically, so it is audited once
// for manual handling and the error keeps it visible on every retry.
func (s *PaymentService) shop01AlreadyProcessed(ctx context.Context, o *dbent.PaymentOrder, tradeNo string, paid float64, pk string) error {
	cur, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
	if err != nil || (cur.Status != OrderStatusExpired && cur.Status != OrderStatusCancelled) {
		return s.alreadyProcessed(ctx, o)
	}
	s.shop01WriteAuditLogOnce(ctx, o.ID, "PAYMENT_AFTER_NOTIFY_WINDOW", pk, map[string]any{
		"status":    cur.Status,
		"createdAt": cur.CreatedAt,
		"tradeNo":   tradeNo,
		"paid":      paid,
		"reason":    "signed payment notify arrived after the notify window; credit manually",
	})
	slog.Error("shop01 payment arrived after notify window; manual handling required",
		"orderID", o.ID, "status", cur.Status, "tradeNo", tradeNo, "paid", paid)
	return fmt.Errorf("shop01 payment for order %d arrived after the %s notify window (status %s); manual handling required",
		o.ID, shop01NotifyWindow, cur.Status)
}

// shop01HandleUnsignedPaidQuery handles a SHOP01 query that reports the order
// paid. The query is unsigned and travels over plain HTTP, so it never credits;
// inside the notify window it only holds off cancel/expiry while the signed
// notify is awaited. The audit row is written once per order.
func (s *PaymentService) shop01HandleUnsignedPaidQuery(ctx context.Context, o *dbent.PaymentOrder, queryRef string, paid float64, pk string) string {
	if s.shop01WriteAuditLogOnce(ctx, o.ID, "PAYMENT_QUERY_PAID_AWAITING_NOTIFY", pk, map[string]any{
		"expected": o.PayAmount,
		"paid":     paid,
		"queryRef": queryRef,
	}) {
		slog.Warn("shop01 query reports paid; awaiting signed notify", "orderID", o.ID, "queryRef", queryRef, "paid", paid)
	}
	if !shop01WithinNotifyWindow(o, time.Now()) {
		// Past the window the order expires normally; a payment is now manual.
		return ""
	}
	return checkPaidResultAlreadyPaid
}

// shop01WriteAuditLogOnce writes an audit row unless one with the same action
// already exists for the order. It reports whether it wrote.
func (s *PaymentService) shop01WriteAuditLogOnce(ctx context.Context, oid int64, action, op string, detail map[string]any) bool {
	exists, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)), paymentauditlog.ActionEQ(action)).
		Exist(ctx)
	if err == nil && exists {
		return false
	}
	s.writeAuditLog(ctx, oid, action, op, detail)
	return true
}

// validateShop01MerchantNoUnique rejects a SHOP01 config whose merchantNo is
// already used by another SHOP01 instance: pay amounts are only kept unique
// per instance, so two instances on one merchant could hand out the same
// amount and the gateway would credit the wrong order.
func (s *PaymentConfigService) validateShop01MerchantNoUnique(ctx context.Context, excludeID int64, cfg map[string]string) error {
	merchantNo := strings.TrimSpace(cfg["merchantNo"])
	if merchantNo == "" {
		return nil
	}
	others, err := s.entClient.PaymentProviderInstance.Query().
		Where(paymentproviderinstance.ProviderKeyEQ(payment.TypeShop01), paymentproviderinstance.IDNEQ(excludeID)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("load shop01 instances: %w", err)
	}
	for _, inst := range others {
		otherCfg, err := s.decryptConfig(inst.Config)
		if err != nil {
			return fmt.Errorf("decrypt shop01 instance %d config: %w", inst.ID, err)
		}
		if strings.TrimSpace(otherCfg["merchantNo"]) == merchantNo {
			return infraerrors.Conflict("SHOP01_MERCHANT_NO_DUPLICATE",
				fmt.Sprintf("merchant number %s is already used by SHOP01 instance %q", merchantNo, inst.Name)).
				WithMetadata(map[string]string{"instance_id": strconv.FormatInt(inst.ID, 10)})
		}
	}
	return nil
}
