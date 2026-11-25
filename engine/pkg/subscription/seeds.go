package subscription

import "github.com/mlz-vpn/mlz-vpn/engine/pkg/model"

// EmbeddedSeeds returns a tiny last-resort pool used when no signed feed has
// ever been accepted (first launch offline or a long CI outage). These are
// public, short-lived proxy endpoints from the same feed; they are never a
// substitute for the signed 5-minute feed and are filtered by Real Delay.
func EmbeddedSeeds() []model.FeedNode {
	return []model.FeedNode{
		{ID: "node-002", Engine: "sing-box", Protocol: "shadowsocks", ConfigURI: "ss://YWVzLTI1Ni1nY206aXR6dnBuQDMyMQ@57.131.27.21:8388#%F0%9F%87%AE%F0%9F%87%B9%20Italy"},
		{ID: "node-003", Engine: "sing-box", Protocol: "vmess", ConfigURI: "vmess://eyJhZGQiOiIxNTQuNDcuMTkuMjI1IiwiYWlkIjoiMCIsImFscG4iOiIiLCJmcCI6IiIsImhvc3QiOiJ4bWZobHB5LmNvbSIsImlkIjoiNWMwNDBmMWQtNDVkYi00M2JkLThkNDYtZTQ5NTBlNDMzNzM1IiwiaW5zZWN1cmUiOiIwIiwibmV0IjoidGNwIiwicGF0aCI6IiIsInBjcyI6IiIsInBvcnQiOiIxMjcwIiwicHMiOiLwn4ep8J+HqvCdmY/wnZma8J2ZofCdmZrwnZmc8J2Zp/CdmZbwnZmi4qu4QPCdmavwnZ+u8J2Zp/CdmZbwnZmu8J2ZifCdmYJf8J2ZiPCdmZbwnZmp8J2ZqPCdmarwnZmn8J2ZniIsInNjeSI6ImF1dG8iLCJzbmkiOiJ4bWZobHB5LmNvbSIsInRscyI6InRscyIsInR5cGUiOiJub25lIiwidiI6IjIiLCJ2Y24iOiIifQ=="},
		{ID: "node-004", Engine: "sing-box", Protocol: "vless", ConfigURI: "vless://d65cc14c-f53f-4fe2-b262-97856601319c@45.144.112.55:443?encryption=none\u0026flow=xtls-rprx-vision\u0026fp=\u0026pbk=e2RLf57Li_-MDZGE9ss1BWPgP54mqRb5PfXhW2jcVVg\u0026security=reality\u0026sid=c39cc7310a\u0026sni=yahoo.com\u0026type=tcp#%F0%9F%8E%8395@oneclickvpnkeys"},
		{ID: "node-005", Engine: "sing-box", Protocol: "hysteria2", ConfigURI: "hy2://57beeeb0-03fc-4fc9-94da-7604488a7134@154.40.49.216:40055/?insecure=1\u0026sni=psstatic.cdn.bcebos.com\u0026obfs=salamander\u0026obfs-password=re61ep4ancwplwh8#ðŸ‡ºðŸ‡¸ 24@exclavevpn"},
		{ID: "node-006", Engine: "sing-box", Protocol: "hysteria2", ConfigURI: "hy2://900c8c92-1979-4c7c-8f2f-63e87d53d8fe@128.254.186.57:13337/?insecure=1\u0026sni=mother-earth.fyi#ðŸ‡¬ðŸ‡§25@exclavevpn"},
	}
}
