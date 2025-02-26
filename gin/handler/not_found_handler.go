package handler

import (
	messagecode "code.finan.one/finan-one-be/fo-utils/config/messagecode"
	"code.finan.one/finan-one-be/fo-utils/gin/response"
	"github.com/gin-gonic/gin"
	"gitlab.com/goxp/cloud0/ginext"
)

func NoRoute(h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (*ginext.Response, error) {
			return h.NewResponse(r.GinCtx, messagecode.GeneralNotFoundCode, nil, nil), nil
		},
	)
}

func NoMethod(h *response.Handler) gin.HandlerFunc {
	return ginext.WrapHandler(
		func(r *ginext.Request) (*ginext.Response, error) {
			return h.NewResponse(r.GinCtx, messagecode.GeneralNotFoundCode, nil, nil), nil
		},
	)
}
