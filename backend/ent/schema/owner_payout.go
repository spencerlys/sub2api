package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OwnerPayout holds the schema definition for the OwnerPayout entity.
//
// AI 账号所有者结算打款记录（USD）。删除策略：软删除，保留审计痕迹。
// owner_user_id 的外键约束由迁移 252 维护，这里不建 edge，避免改动 User schema。
type OwnerPayout struct {
	ent.Schema
}

func (OwnerPayout) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "owner_payouts"},
	}
}

func (OwnerPayout) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.SoftDeleteMixin{},
	}
}

func (OwnerPayout) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id").
			Comment("所有者用户ID"),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Comment("打款金额（USD）"),
		field.Time("paid_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("打款时间"),
		field.String("notes").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("created_by").
			Optional().
			Nillable().
			Comment("记录打款的管理员用户ID"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OwnerPayout) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id", "paid_at"),
		index.Fields("deleted_at"),
	}
}
