package middleware

import (
	"context"

	"code.finan.one/finan-one-be/fo-utils/gin/response"
	"code.finan.one/finan-one-be/fo-utils/net/uthttp"
	"code.finan.one/finan-one-be/fo-utils/utils/utfunc"
	"github.com/gin-gonic/gin"
	"gitlab.com/goxp/cloud0/ginext"
	"gitlab.com/goxp/cloud0/logger"
)

func SetLocaleToCTX() gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (res *ginext.Response, err error) {
			c := r.GinCtx
			request := c.Request
			ctx := request.Context()
			locale := uthttp.GetLocaleFromHeader(request)

			ctx = context.WithValue(ctx, uthttp.HeaderLocale, locale)
			request = request.WithContext(ctx)
			c.Request = request
			c.Next()

			return nil, nil
		},
	)
}

func SetRequestIDToCTX(h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (res *ginext.Response, err error) {
			log := logger.WithCtx(r.Context(), utfunc.GetCurrentCaller(h, 0))

			c := r.GinCtx
			request := c.Request
			ctx := request.Context()
			requestID, err := uthttp.GetRequestIDFromHeader(request)
			if err != nil {
				log.WithError(err).Warn("request id not found")
				c.Abort()
				return response.GeneralBadRequestResponse(err)
			}

			ctx = context.WithValue(ctx, uthttp.HeaderRequestID, requestID)
			request = request.WithContext(ctx)
			c.Request = request
			c.Next()

			return nil, nil
		},
	)
}

func SetUserIDToCTX(h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (res *ginext.Response, err error) {
			c := r.GinCtx
			request := c.Request
			ctx := request.Context()
			userID, err := uthttp.GetUserIDFromHeader(request)
			if err != nil {
				c.Next()
				return nil, nil
			}

			ctx = context.WithValue(ctx, uthttp.HeaderUserID, userID.String())
			request = request.WithContext(ctx)
			c.Request = request
			c.Next()

			return nil, nil
		},
	)
}
