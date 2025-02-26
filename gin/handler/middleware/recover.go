package middleware

import (
	messagecode "code.finan.one/finan-one-be/fo-utils/config/messagecode"
	"code.finan.one/finan-one-be/fo-utils/gin/response"

	"github.com/gin-gonic/gin"
	"gitlab.com/goxp/cloud0/ginext"
)

func Recover(h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (res *ginext.Response, err error) {
			defer func() {
				err := recover()
				if err != nil {
					r.GinCtx.Header("Connection", "close")
					res = h.NewResponse(r.GinCtx, messagecode.GeneralInternalErrorCode, nil, nil)
				}
			}()
			c := r.GinCtx
			c.Next()

			return nil, nil
		},
	)
}
