package viettelpay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"time"

	"giautm.dev/viettelpay/soap"
	"github.com/oklog/ulid"
)

func DefaultGenID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

type CheckAccount struct {
	MSISDN       string `json:"msisdn"`
	CustomerName string `json:"customerName"`
}

type CheckAccountResponse struct {
	CheckAccount
	Package string `json:"package"`
	Code    string `json:"errorCode"`
	Message string `json:"errorMsg"`
}

type RequestPayment struct {
	TransactionID string `json:"transId"`
	MSISDN        string `json:"msisdn"`
	CustomerName  string `json:"customerName"`
	Amount        int64  `json:"amount"`
	SMSContent    string `json:"smsContent"`
	Note          string `json:"note"`
}

type RequestPaymentResponse struct {
	RequestPayment
	Code    string `json:"errorCode"`
	Message string `json:"errorMsg"`
}

type RequestPaymentEnvelope struct {
	EnvelopeBase
	TotalAmount        int64  `json:"totalAmount"`
	TotalTransactions  int    `json:"totalTrans"`
	TransactionContent string `json:"transContent"`
}

type PartnerAPI interface {
	Process(ctx context.Context, req Request, response interface{}) error
	CheckAccount(ctx context.Context, checks ...CheckAccount) ([]CheckAccountResponse, error)
	RequestPayment(ctx context.Context, transactionContent string, reqs ...RequestPayment) ([]RequestPaymentResponse, error)
}

type options struct {
	password    string
	username    string
	serviceCode string
	genID       func() string
}

// A Option sets options such as credentials, tls, etc.
type Option func(*options)

// WithAuth is an Option to set BasicAuth
func WithAuth(username, password, serviceCode string) Option {
	return func(o *options) {
		o.username = username
		o.password = password
		o.serviceCode = serviceCode
	}
}

var ns2Opt = soap.WithNS2("http://partnerapi.bankplus.viettel.com/")

var defaultOptions = options{
	username:    os.Getenv("VIETTELPAY_USERNAME"),
	password:    os.Getenv("VIETTELPAY_PASSWORD"),
	serviceCode: os.Getenv("VIETTELPAY_SERVICE_CODE"),
	genID:       DefaultGenID,
}

type partnerAPI struct {
	client *soap.Client
	opts   *options

	PartnerPrivateKey *rsa.PrivateKey
	ViettelPublicKey  *rsa.PublicKey
}

func NewPartnerAPI(ctx context.Context, url string, opt ...Option) (_ PartnerAPI, err error) {
	opts := defaultOptions
	for _, o := range opt {
		o(&opts)
	}

	var (
		prikey *rsa.PrivateKey
		pubkey *rsa.PublicKey
	)
	if prikey, err = partnerKey(ctx, "file:///workspaces/viettelpay/keys/partner-private-key.pem?decoder=bytes"); err != nil {
		return nil, err
	}

	if pubkey, err = viettelKey(ctx, "file:///workspaces/viettelpay/keys/viettel-public-key.pem?decoder=bytes"); err != nil {
		return nil, err
	}

	return &partnerAPI{
		client: soap.NewClient(url, ns2Opt),
		opts:   &opts,

		PartnerPrivateKey: prikey,
		ViettelPublicKey:  pubkey,
	}, nil
}

func (s *partnerAPI) CheckAccount(ctx context.Context, checks ...CheckAccount) ([]CheckAccountResponse, error) {
	results := []CheckAccountResponse{}
	err := s.Process(ctx, NewRequest("VTP305", checks, nil), &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *partnerAPI) RequestPayment(ctx context.Context, transactionContent string, reqs ...RequestPayment) ([]RequestPaymentResponse, error) {
	env := &RequestPaymentEnvelope{
		TotalTransactions:  len(reqs),
		TransactionContent: transactionContent,
	}
	for _, v := range reqs {
		env.TotalAmount += v.Amount
	}

	results := []RequestPaymentResponse{}
	err := s.Process(ctx, NewRequest("VTP306", reqs, env), &results)
	if err != nil {
		return nil, err
	}
	return results, nil
}
