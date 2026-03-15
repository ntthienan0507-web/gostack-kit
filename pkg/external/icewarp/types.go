package icewarp

import "encoding/xml"

// SendRequest represents an email send request for the IceWarp API.
type SendRequest struct {
	XMLName xml.Name `xml:"SendRequest"`
	From    string   `xml:"From"`
	To      string   `xml:"To"`
	CC      string   `xml:"CC,omitempty"`
	BCC     string   `xml:"BCC,omitempty"`
	Subject string   `xml:"Subject"`
	Body    Body     `xml:"Body"`
}

// Body represents the email body with a content type attribute.
type Body struct {
	ContentType string `xml:"contentType,attr"`
	Content     string `xml:",chardata"`
}

// SendResponse represents the response from sending an email.
type SendResponse struct {
	XMLName   xml.Name `xml:"SendResponse"`
	MessageID string   `xml:"MessageID"`
	Status    string   `xml:"Status"`
}

// AccountInfo represents IceWarp account information.
type AccountInfo struct {
	XMLName     xml.Name `xml:"AccountInfo"`
	Email       string   `xml:"Email"`
	DisplayName string   `xml:"DisplayName"`
	Quota       int64    `xml:"Quota"`
	UsedQuota   int64    `xml:"UsedQuota"`
	IsActive    bool     `xml:"IsActive"`
}

// CreateAccountRequest represents a request to create a new IceWarp account.
type CreateAccountRequest struct {
	XMLName     xml.Name `xml:"CreateAccountRequest"`
	Email       string   `xml:"Email"`
	DisplayName string   `xml:"DisplayName"`
	Password    string   `xml:"Password"`
	Quota       int64    `xml:"Quota,omitempty"`
}

// CreateAccountResponse represents the response from creating an account.
type CreateAccountResponse struct {
	XMLName xml.Name `xml:"CreateAccountResponse"`
	Email   string   `xml:"Email"`
	Status  string   `xml:"Status"`
}
