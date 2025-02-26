package middleware

import (
	"errors"

	messagecode "code.finan.one/finan-one-be/fo-utils/config/messagecode"
	"code.finan.one/finan-one-be/fo-utils/gin/response"

	"github.com/gin-gonic/gin"
	"gitlab.com/goxp/cloud0/ginext"
)

func HandleError(handler *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (*ginext.Response, error) {
			c := r.GinCtx
			c.Next()
			if len(c.Errors) > 0 {
				err := c.Errors[0]
				//c.Errors = nil
				var messCodeErr messagecode.Error
				var serviceErr messagecode.ServiceError
				switch {
				case errors.As(err, &messCodeErr):
					return handler.NewResponse(r.GinCtx, messCodeErr.Code, nil, nil, messCodeErr.Params...), nil
				case errors.As(err, &serviceErr):
					return handler.NewRawResponse(c, serviceErr.Code, serviceErr.Message.Content, serviceErr.Data, serviceErr.Meta, serviceErr.Message.Params...), nil
				default:
					return handler.NewResponse(r.GinCtx, messagecode.GeneralInternalErrorCode, nil, nil), nil
				}
			}

			return nil, nil
		},
	)
}
