package utcontext

import (
	"context"
	"errors"

	valueobject "code.finan.one/finan-one-be/fo-utils/model/value-object"

	"code.finan.one/finan-one-be/fo-utils/net/uthttp"
)

var (
	ErrNotFound = errors.New("value not found in context")
)

func SetAuthInfoToContext(ctx context.Context, authInfo *valueobject.Auth) context.Context {
	ctx = context.WithValue(ctx, uthttp.HeaderRequestID, authInfo.RequestID)
	ctx = context.WithValue(ctx, uthttp.HeaderUserID, authInfo.UserID)
	ctx = context.WithValue(ctx, uthttp.HeaderBusinessID, authInfo.BusinessID)
	ctx = context.WithValue(ctx, uthttp.HeaderOrgID, authInfo.OrgID)
	ctx = context.WithValue(ctx, uthttp.HeaderLocale, authInfo.Locale)
	ctx = context.WithValue(ctx, uthttp.HeaderTimezone, authInfo.Timezone)
	return ctx
}

func GetUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(uthttp.HeaderUserID).(string)
	if userID == "" || !ok {
		return "", ErrNotFound
	}

	return userID, nil
}

func GetRequestIDFromContext(ctx context.Context) (string, error) {
	requestID, ok := ctx.Value(uthttp.HeaderRequestID).(string)
	if requestID == "" || !ok {
		return "", ErrNotFound
	}

	return requestID, nil
}

func GetLocaleFromContext(ctx context.Context) (string, error) {
	locale, ok := ctx.Value(uthttp.HeaderLocale).(string)
	if locale == "" || !ok {
		return "", ErrNotFound
	}

	return locale, nil
}

func GetBackgroundContext(ctx context.Context) context.Context {
	bgCtx := context.Background()

	requestID, _ := GetRequestIDFromContext(ctx)
	if requestID == "" {
		return bgCtx
	}
	bgCtx = context.WithValue(bgCtx, uthttp.HeaderRequestID, requestID)

	return bgCtx
}
