package httpapi

import (
	"context"

	"github.com/ch00z00/uchinokusa/server/internal/adapter/httpapi/oapi"
	"github.com/ch00z00/uchinokusa/server/internal/domain"
)

// server は、API 定義から生成したインターフェース（oapi.StrictServerInterface）を実装する。
// 各メソッドは、成功のレスポンスを返すか、ドメインのエラーを返す（エラーは writeError が変換する）。
type server struct {
	accounts AccountService
}

var _ oapi.StrictServerInterface = (*server)(nil)

func (s *server) GetMe(ctx context.Context, _ oapi.GetMeRequestObject) (oapi.GetMeResponseObject, error) {
	return oapi.GetMe200JSONResponse(toMe(currentUser(ctx))), nil
}

func (s *server) UpdateMe(ctx context.Context, req oapi.UpdateMeRequestObject) (oapi.UpdateMeResponseObject, error) {
	u, err := s.accounts.UpdateDisplayName(ctx, currentUser(ctx).ID, req.Body.DisplayName)
	if err != nil {
		return nil, err
	}
	return oapi.UpdateMe200JSONResponse(toMe(u)), nil
}

func (s *server) RegisterDevice(ctx context.Context, req oapi.RegisterDeviceRequestObject) (oapi.RegisterDeviceResponseObject, error) {
	d, err := s.accounts.RegisterDevice(ctx, currentUser(ctx).ID, req.Body.FcmToken, req.Body.Platform, req.Body.TimeZone)
	if err != nil {
		return nil, err
	}
	return oapi.RegisterDevice200JSONResponse(oapi.Device{
		Id:         d.ID,
		Platform:   oapi.Platform(d.Platform),
		TimeZone:   d.TimeZone,
		LastSeenAt: d.LastSeenAt.UTC(),
	}), nil
}

func (s *server) UnregisterDevice(ctx context.Context, req oapi.UnregisterDeviceRequestObject) (oapi.UnregisterDeviceResponseObject, error) {
	if err := s.accounts.UnregisterDevice(ctx, currentUser(ctx).ID, req.DeviceId); err != nil {
		return nil, err
	}
	return oapi.UnregisterDevice204Response{}, nil
}

func toMe(u domain.User) oapi.Me {
	return oapi.Me{
		Id:          u.ID,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt.UTC(), // 日時は UTC で返す（API の取り決め）
	}
}
