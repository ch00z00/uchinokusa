package httpapi

import "github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"

// server は、API 定義から生成したインターフェース（oapi.StrictServerInterface）を実装する。
// エンドポイントのメソッドは、タスク 2.1 以降で追加する。
type server struct{}

var _ oapi.StrictServerInterface = (*server)(nil)
