// 再帰リゾルバの実装。
// ルートサーバーから権威サーバーを辿ってレコードを解決する。
package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MASAKi-cell/dns/client"
	"github.com/MASAKi-cell/dns/message"
)

// 再帰解決の最大深さ。
const maxDepth = 16

// 再帰リゾルバ。
type Resolver struct {
	cache  *Cache
	client *client.Client
}

// 新しいリゾルバを作成する。
func NewResolver() *Resolver {
	return &Resolver{
		cache: NewCache(),
		client: client.NewClient(
			client.WithTimeout(2*time.Second),
			client.WithMaxRetries(2),
		),
	}
}

// 指定した名前とタイプのレコードを解決する。
func (r *Resolver) Resolve(ctx context.Context, name string, typ message.Type) ([]message.ResourceRecord, error) {
	// FQDN形式に正規化
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	// キャッシュを確認
	if records := r.cache.Get(name, typ); records != nil {
		return records, nil
	}

	// ルートサーバーから解決開始
	records, err := r.resolve(ctx, name, typ, RootServers, 0)
	if err != nil {
		return nil, err
	}

	// 結果をキャッシュ
	r.cache.Set(records)

	return records, nil
}

func (r *Resolver) resolve(ctx context.Context, name string, typ message.Type, servers []string, depth int) ([]message.ResourceRecord, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("max recursion depth exceeded")
	}

	// サーバーにクエリを送信
	resp, err := r.query(ctx, name, typ, servers)
	if err != nil {
		return nil, err
	}

	// 応答を解析
	switch {
	case resp.Header.RCode == message.RCodeNameError:
		// NXDOMAIN
		return nil, fmt.Errorf("NXDOMAIN: %s", name)

	case resp.Header.RCode != message.RCodeSuccess:
		return nil, fmt.Errorf("query failed: %s", resp.Header.RCode)

	case len(resp.Answers) > 0:
		// 応答あり
		// CNAMEの場合は追跡
		for _, ans := range resp.Answers {
			if ans.Type == message.TypeCNAME && typ != message.TypeCNAME {
				if cnameData, ok := ans.RData.(message.CNAMEData); ok {
					target := string(cnameData.CName)
					// CNAMEの先を解決
					targetRecords, err := r.Resolve(ctx, target, typ)
					if err != nil {
						return nil, err
					}
					// CNAMEと解決結果を返す
					result := make([]message.ResourceRecord, 0, len(resp.Answers)+len(targetRecords))
					result = append(result, resp.Answers...)
					result = append(result, targetRecords...)
					return result, nil
				}
			}
		}

		// キャッシュに追加
		r.cache.Set(resp.Answers)
		return resp.Answers, nil

	case len(resp.Authorities) > 0:
		// リファラル（委譲）
		nextServers := r.extractNextServers(resp)
		if len(nextServers) == 0 {
			return nil, fmt.Errorf("no nameservers found in referral")
		}
		return r.resolve(ctx, name, typ, nextServers, depth+1)

	default:
		// 応答なし
		return nil, fmt.Errorf("no answer for %s %s", name, typ)
	}
}

// サーバーにクエリを送信する。
func (r *Resolver) query(ctx context.Context, name string, typ message.Type, servers []string) (*message.Message, error) {
	c := client.NewClient(
		client.WithServers(servers...),
		client.WithTimeout(2*time.Second),
		client.WithMaxRetries(1),
	)

	return c.Query(ctx, name, typ)
}

// リファラル応答から次のネームサーバーのアドレスを抽出する。
func (r *Resolver) extractNextServers(resp *message.Message) []string {
	var servers []string

	// Authorityセクションからネームサーバー名を取得
	var nsNames []string
	for _, auth := range resp.Authorities {
		if auth.Type == message.TypeNS {
			if nsData, ok := auth.RData.(message.NSData); ok {
				nsNames = append(nsNames, string(nsData.NSDName))
			}
		}
	}

	// AdditionalセクションからIPアドレスを取得（グルーレコード）
	nsAddrs := make(map[string]string)
	for _, add := range resp.Additionals {
		if add.Type == message.TypeA {
			if aData, ok := add.RData.(message.AData); ok {
				addr := fmt.Sprintf("%d.%d.%d.%d:53",
					aData.Address[0], aData.Address[1],
					aData.Address[2], aData.Address[3])
				nsAddrs[strings.ToLower(string(add.Name))] = addr
			}
		}
	}

	// NSのアドレスを解決
	for _, nsName := range nsNames {
		nsNameLower := strings.ToLower(nsName)
		if addr, ok := nsAddrs[nsNameLower]; ok {
			servers = append(servers, addr)
		}
	}

	// グルーレコードがない場合は、キャッシュから探す
	if len(servers) == 0 {
		for _, nsName := range nsNames {
			if cached := r.cache.Get(nsName, message.TypeA); cached != nil {
				for _, rr := range cached {
					if aData, ok := rr.RData.(message.AData); ok {
						addr := fmt.Sprintf("%d.%d.%d.%d:53",
							aData.Address[0], aData.Address[1],
							aData.Address[2], aData.Address[3])
						servers = append(servers, addr)
					}
				}
			}
		}
	}

	// それでも見つからない場合、NSの名前解決を試みる
	if len(servers) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, nsName := range nsNames {
			records, err := r.Resolve(ctx, nsName, message.TypeA)
			if err != nil {
				continue
			}
			for _, rr := range records {
				if aData, ok := rr.RData.(message.AData); ok {
					addr := fmt.Sprintf("%d.%d.%d.%d:53",
						aData.Address[0], aData.Address[1],
						aData.Address[2], aData.Address[3])
					servers = append(servers, addr)
				}
			}
			if len(servers) > 0 {
				break
			}
		}
	}

	return servers
}

// キャッシュを返す（テスト・デバッグ用）。
func (r *Resolver) Cache() *Cache {
	return r.cache
}
