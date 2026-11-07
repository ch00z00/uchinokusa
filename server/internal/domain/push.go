package domain

// PushMessage は、端末に送るプッシュ通知である。
type PushMessage struct {
	Title string
	Body  string
	// Data は、通知をタップしたときにアプリが使う値である（開く画面の世帯・植物の ID など）。FCM の data は文字列だけを持てる
	Data map[string]string
}

// PushOutcome は、1つの通知トークンへの送信の結果である。
type PushOutcome string

const (
	PushDelivered    PushOutcome = "delivered"    // FCM が受け付けた
	PushUnregistered PushOutcome = "unregistered" // トークンが無効になった（アプリの削除など）。トークンを消す
	PushFailed       PushOutcome = "failed"       // 一時的な失敗など。トークンは消さない
)

// PushReport は、ユーザーの端末への送信の結果の件数である。
type PushReport struct {
	Delivered int
	Failed    int
	Removed   int // 無効になったので消したトークンの数
}
