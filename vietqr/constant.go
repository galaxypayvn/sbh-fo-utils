package qr

const (
	PAYLOAD_FORMAT_INDICATOR   = "00"
	POINT_OF_INITIATION_METHOD = "01"
	//consumer
	CONSUMER_ACCOUNT_INFORMATION = "38"
	AID                          = "00"
	CONSUMER_SUB_INFO            = "01"
	BNB_CONSUMER                 = "01"
	BNBID                        = "00"
	CONSUMER_ID                  = "01"
	SERVICE_ID                   = "02"
	//transaction
	TRANSACTION_CURRENCY           = "53"
	COUNTRY_CODE                   = "58"
	TRANSACTION_AMOUNT             = "54"
	ADDITIONAL_DATA_FIELD_TEMPlATE = "62"
	CRCID                          = "63"
)

const (
	JPY = "392"
	KRW = "410"
	MYR = "458"
	CNY = "156"
	IDR = "360"
	PHP = "608"
	SGD = "702"
	THB = "764"
	VND = "704"
)

var currentCodeCache = map[string]string{
	"JPY": "392",
	"KRW": "410",
	"MYR": "458",
	"CNY": "156",
	"IDR": "360",
	"PHP": "608",
	"SGD": "702",
	"THB": "764",
	"VND": "704",
}

const (
	T_BILL_NUMBER                       = "01"
	T_MOBILE_NUMBER                     = "02"
	T_STORE_LABEL                       = "03"
	T_LOYALTY_NUMBER                    = "04"
	T_REFERENCE_LABLE                   = "05"
	T_CUSTOMER_LABLE                    = "06"
	T_TERMINAL_LABLE                    = "07"
	T_PURPOSE_OF_TRANSACTION            = "08"
	T_ADDITIONAL_CONSUMER_DATA_REQUEST  = "09"
	T_RFU_FOR_EMVCo                     = "10"
	T_PAYMENT_SYSTEM_SPECIFIC_TEMPLATES = "11"
)
