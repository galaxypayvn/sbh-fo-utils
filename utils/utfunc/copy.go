package utfunc

import (
	"time"

	"code.finan.one/finan-one-be/fo-utils/utils/customtype"
	"github.com/jinzhu/copier"
)

var customTimeToTimeConverters = []copier.TypeConverter{
	{
		SrcType: customtype.Time{},
		DstType: time.Time{},
		Fn: func(src interface{}) (interface{}, error) {
			customTm, ok := src.(customtype.Time)
			if !ok {
				return time.Time{}, nil
			}

			return customTm.Time.UTC(), nil
		},
	},
	{
		SrcType: &customtype.Time{},
		DstType: &time.Time{},
		Fn: func(src interface{}) (interface{}, error) {
			customTm, ok := src.(*customtype.Time)
			if !ok {
				return time.Time{}, nil
			}

			if customTm == nil {
				return nil, nil
			}

			tm := customTm.Time.UTC()
			return &tm, nil
		},
	},
	{
		SrcType: customtype.Time{},
		DstType: &time.Time{},
		Fn: func(src interface{}) (interface{}, error) {
			customTm, ok := src.(customtype.Time)
			if !ok {
				return nil, nil
			}

			tm := customTm.Time.UTC()
			return &tm, nil
		},
	},
}

func CopyWhenUpdate(toValue any, fromValue any) error {
	return copier.CopyWithOption(toValue, fromValue, copier.Option{
		IgnoreEmpty: true,
		Converters:  customTimeToTimeConverters,
	})
}

func CopyWhenInsert(toValue any, fromValue any) error {
	return copier.CopyWithOption(toValue, fromValue, copier.Option{
		IgnoreEmpty: true,
		Converters:  customTimeToTimeConverters,
	})
}
