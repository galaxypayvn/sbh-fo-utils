package qr

type Bank struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Code              string `json:"code"`
	Bin               string `json:"bin"`
	ShortName         string `json:"shortName"`
	Logo              string `json:"logo"`
	TransferSupported int    `json:"transferSupported"`
	LookupSupported   int    `json:"lookupSupported"`
	ShortName0        string `json:"short_name"`
	Support           int    `json:"support"`
	IsTransfer        int    `json:"isTransfer"`
	SwiftCode         string `json:"swift_code"`
}

type DataInquirySample struct {
	FullName      string `json:"fullName"`
	PhoneNumber   string `json:"phoneNumber"`
	PersonalId    string `json:"personalId"`
	AccountNumber string `json:"accountNumber"`
	BankCode      string `json:"bankCode"`
}
