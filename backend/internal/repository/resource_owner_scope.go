package repository

import (
	"context"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbgroup "github.com/Wei-Shaw/sub2api/ent/group"
	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
	dbproxy "github.com/Wei-Shaw/sub2api/ent/proxy"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
)

// resourceOwnerID 返回 AI 账号所有者请求中的用户 ID，用于登记新建资源的归属；
// 管理员与后台任务返回 nil，资源归管理员所有。
func resourceOwnerID(ctx context.Context) *int64 {
	if id := adminaccess.OwnerUserID(ctx); id > 0 {
		return &id
	}
	return nil
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
