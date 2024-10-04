package qr

import (
	"code.finan.cc/finan-one-be/fo-utils/valid"
	"fmt"
)

/*   ------------ SAMPLE ------------
var input = qr.QRInput{
	PayloadFormatIndicator:  "000201",
	PointOfInitiationMethod: pointOfInitiationMethod, //dynamic QR : 12 - static QR 11
	ConsumerAccountInfomation: qr.ConsumerAccountInfomation{
		BankCode:   req.BankCode,
		AID:        "A000000727",
		ConsumerID: req.ConsumerID,
		ServiceID:  "QRIBFTTA",
	},
	TypeAddition: "08",
	CurrentCode:  "VND",
	CountryCode:  "VN",
}
input.TransactionAmount = transactionAmount
input.AdditionalDataFieldTemplate = &addDataField
resp.QRCode = input.String()
*/

type QRInput struct {
	PayloadFormatIndicator      string                    `json:"payload_format_indicator"`
	PointOfInitiationMethod     string                    `json:"point_of_initiation_method"`
	ConsumerAccountInfomation   ConsumerAccountInfomation `json:"consumer_account_infomation"`
	CurrentCode                 string                    `json:"current_code"`
	CountryCode                 string                    `json:"country_code"`
	TransactionAmount           *int64                    `json:"transaction_amount,omitempty"`
	TypeAddition                string                    `json:"type_addition"`
	AdditionalDataFieldTemplate *string                   `json:"additional_data_field_template,omitempty"`
	CRC                         string                    `json:"crc,omitempty"`
}

type ConsumerAccountInfomation struct {
	BankCode   string `json:"bank_code"`
	AID        string `json:"aid"` //NAPAS:A000000727
	ConsumerID string `json:"consumer_id"`
	ServiceID  string `json:"service_id"` //QRIBFTTC: ck-> the ,QRIBFTTA: ck->tk
}

func GenerateQR(
	binCode string,
	accountNumber, dataField *string,
	amount *int64) QRInput {
	var input = QRInput{
		PayloadFormatIndicator: "000201",
		PointOfInitiationMethod: func() string {
			if nil != amount && *amount == 0 {
				return "010211"
			}
			return "010212"
		}(),
		ConsumerAccountInfomation: ConsumerAccountInfomation{
			BankCode:   binCode,
			ConsumerID: valid.String(accountNumber),
			AID:        "A000000727",
			ServiceID:  "QRIBFTTA",
		},
		TypeAddition:                "08",
		CurrentCode:                 "VND",
		CountryCode:                 "VN",
		TransactionAmount:           amount,
		AdditionalDataFieldTemplate: dataField,
	}
	return input
}

func (s QRInput) String() string {
	var transactionAmount, additionalDataFieldTemplate string
	if s.TransactionAmount != nil {
		var transactionAmountStr = fmt.Sprintf("%d", *s.TransactionAmount)
		transactionAmount = fmt.Sprintf("%s%s%s", TRANSACTION_AMOUNT, lenghtStr(transactionAmountStr), transactionAmountStr)
	}
	if s.AdditionalDataFieldTemplate != nil {
		var tmp = s.TypeAddition + lenghtStr(*s.AdditionalDataFieldTemplate) + *s.AdditionalDataFieldTemplate

		additionalDataFieldTemplate = fmt.Sprintf("%s%s%s", ADDITIONAL_DATA_FIELD_TEMPlATE, lenghtStr(tmp), tmp)
	}
	tranCurrentCode := currentCodeCache[s.CurrentCode]
	transactionCurrency := fmt.Sprintf("%s%s%s", TRANSACTION_CURRENCY, lenghtStr(tranCurrentCode), tranCurrentCode)
	s.CountryCode = fmt.Sprintf("%s%s%s", COUNTRY_CODE, lenghtStr(s.CountryCode), s.CountryCode)

	return gen(s.PayloadFormatIndicator, s.PointOfInitiationMethod, s.ConsumerAccountInfomation.String(), transactionCurrency, transactionAmount, s.CountryCode, additionalDataFieldTemplate)
}

func (c ConsumerAccountInfomation) String() string {
	//bnbID := getBinBank(c.BankCode)
	bnbID := c.BankCode
	bnbConvert := fmt.Sprintf("%s%s%v", BNBID, lenghtStr(bnbID), bnbID)
	consumerIDConvert := fmt.Sprintf("%s%s%v", CONSUMER_ID, lenghtStr(c.ConsumerID), c.ConsumerID)
	consumerAccountInfomationBase := fmt.Sprintf("%v%v", bnbConvert, consumerIDConvert)
	serviceConvert := fmt.Sprintf("%s%s%v", SERVICE_ID, lenghtStr(c.ServiceID), c.ServiceID)
	consumerInfomationBase1 := fmt.Sprintf("%s%s%s%s", CONSUMER_SUB_INFO, lenghtStr(consumerAccountInfomationBase), consumerAccountInfomationBase, serviceConvert)
	consumerInfo := fmt.Sprintf("%v%s%v%v", AID, lenghtStr(c.AID), c.AID, consumerInfomationBase1)
	consumerAccountInfomationBase2 := fmt.Sprintf("%s%s%s", CONSUMER_ACCOUNT_INFORMATION, lenghtStr(consumerInfo), consumerInfo)
	return consumerAccountInfomationBase2
}

func lenghtStr(val string) string {
	var count = len(val)
	if count < 10 {
		return fmt.Sprintf("%s%d", "0", count)
	}
	return fmt.Sprintf("%d", count)
}

func gen(vals ...string) string {
	var info string
	for _, item := range vals {
		info += item
	}
	info += "6304"
	crc16 := CalculateCRC(CCITT, []byte(info))
	return fmt.Sprintf("%s%v", info, fmt.Sprintf("%04X", crc16))
}
