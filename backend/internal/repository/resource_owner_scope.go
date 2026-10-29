package repository

import (
	"context"
	"fmt"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbgroup "github.com/Wei-Shaw/sub2api/ent/group"
	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
	dbproxy "github.com/Wei-Shaw/sub2api/ent/proxy"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// resourceOwnerID 返回新建资源应登记的归属：AI 账号所有者请求取本人，复制/影子取服务端
// 显式继承的来源归属；其余（管理员与后台任务）返回 nil，资源归管理员所有。
func resourceOwnerID(ctx context.Context) *int64 {
	return service.ResourceOwnerForCreate(ctx)
}

// 以下谓词在所有者请求中把列表限定为本人资源，其他身份返回空切片不追加条件。
func ownerAccountScope(ctx context.Context) []dbpredicate.Account {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		return []dbpredicate.Account{dbaccount.OwnerUserIDEQ(id)}
	}
	return nil
}

func ownerGroupScope(ctx context.Context) []dbpredicate.Group {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		return []dbpredicate.Group{dbgroup.OwnerUserIDEQ(id)}
	}
	return nil
}

func ownerProxyScope(ctx context.Context) []dbpredicate.Proxy {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		return []dbpredicate.Proxy{dbproxy.OwnerUserIDEQ(id)}
	}
	return nil
}

// adminOwnedAccountPredicate 限定为管理员所有的账户。所有者账户只能通过分组绑定进入调度，
// 未分组池与简易模式的全平台池（及据此生成的调度快照）必须排除它们，否则会被未授权的用户流量选中。
func adminOwnedAccountPredicate() dbpredicate.Account {
	return dbaccount.OwnerUserIDIsNil()
}

// ResourceOwnerIDs 批量读取账户、分组或代理的归属（id → owner_user_id），每类一次查询且只取两列。
// 不附加所有者列表范围，专供归属校验；不存在或已软删除的 id 不出现在结果中。
func (r *accountRepository) ResourceOwnerIDs(ctx context.Context, kind string, ids []int64) (map[int64]*int64, error) {
	ids = uniquePositiveInt64s(ids)
	out := make(map[int64]*int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	switch kind {
	case service.OwnerResourceAccount:
		rows, err := r.client.Account.Query().Where(dbaccount.IDIn(ids...)).Select(dbaccount.FieldID, dbaccount.FieldOwnerUserID).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.ID] = row.OwnerUserID
		}
	case service.OwnerResourceGroup:
		rows, err := r.client.Group.Query().Where(dbgroup.IDIn(ids...)).Select(dbgroup.FieldID, dbgroup.FieldOwnerUserID).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.ID] = row.OwnerUserID
		}
	case service.OwnerResourceProxy:
		rows, err := r.client.Proxy.Query().Where(dbproxy.IDIn(ids...)).Select(dbproxy.FieldID, dbproxy.FieldOwnerUserID).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.ID] = row.OwnerUserID
		}
	default:
		return nil, fmt.Errorf("unknown owner resource kind: %s", kind)
	}
	return out, nil
}
