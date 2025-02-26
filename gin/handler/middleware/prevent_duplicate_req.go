package middleware

import (
	"fmt"
	"time"

	messagecode "code.finan.one/finan-one-be/fo-utils/config/messagecode"
	"code.finan.one/finan-one-be/fo-utils/gin/response"

	"code.finan.one/finan-one-be/fo-utils/net/uthttp"
	"code.finan.one/finan-one-be/fo-utils/sdk/redis"
	"github.com/gin-gonic/gin"
	"gitlab.com/goxp/cloud0/ginext"
)

func PreventDuplicateReq(cache redis.IRedisRepo, h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (*ginext.Response, error) {
			c := r.GinCtx
			clientRequestID, err := uthttp.GetClientRequestIDFromHeader(c.Request)
			if err != nil {
				c.Next()
				return nil, nil
			}

			deviceID, err := uthttp.GetDeviceIDFromHeader(c.Request)
			if err != nil {
				c.Next()
				return nil, nil
			}

			key := makeClientRequestKey(clientRequestID, deviceID)

			found, err := cache.CheckExist(r.Context(), key)
			if err != nil {
				c.Next()
				return nil, nil
			}

			if found {
				c.Abort()
				return h.NewResponse(c, messagecode.GeneralDuplicateRequestCode, nil, nil), nil
			} else {
				_ = cache.SetKey(r.Context(), key, 1, 10*time.Second)
			}
			c.Next()

			return nil, nil
		},
	)
}

func makeClientRequestKey(clientRequestID string, deviceID string) string {
	return fmt.Sprintf("client-request-id:%s:%s", clientRequestID, deviceID)
}
