package l

import (
	"testing"

	"code.finan.one/finan-one-be/fo-utils/l/sentry"
)

func TestNew(t *testing.T) {

	//ll = New()
	ll = NewWithSentry(&sentry.Configuration{
		DSN:   "https://6c823523782944c597fcc102c8b6ae4e@o390151.ingest.sentry.io/5231166",
		Trace: struct{ Disabled bool }{Disabled: false},
	})
	defer ll.Sync()
	a := map[string]interface{}{
		"testdebug": 1,
	}
	ll.Debug("example.ping debug", Any("example.ping debug", a))
	ll.Info("example.ping info", Any("example.ping debug", a))
	ll.Warn("example.ping warn")
	//ll.Panic("fatal")
	ll.Error("example.ping err")

}
