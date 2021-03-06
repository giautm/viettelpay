package viettelpay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
)

type EnvelopeBase struct {
	Password    string `json:"password"`
	ServiceCode string `json:"serviceCode"`
	Username    string `json:"username"`

	Data    []byte `json:"data,omitempty"`
	OrderID string `json:"orderId"`
}

func (e *EnvelopeBase) SetData(val []byte) {
	e.Data = val
}
func (e *EnvelopeBase) SetUsername(val string) {
	e.Username = val
}
func (e *EnvelopeBase) SetPassword(val string) {
	e.Password = val
}
func (e *EnvelopeBase) SetServiceCode(val string) {
	e.ServiceCode = val
}

type EnvelopeResponse struct {
	Data      json.RawMessage `json:"data"`
	Signature []byte          `json:"signature"`
}

type EnvelopeResponseData struct {
	Data    []byte `json:"data,omitempty"`
	OrderID string `json:"orderId"`

	RealServiceCode string `json:"realServiceCode"`
	ServiceCode     string `json:"serviceCode"`
	Username        string `json:"username"`

	RequestId string `json:"requestId"`
	TransDate string `json:"transDate"`

	BatchErrorCode string `json:"batchErrorCode"`
	BatchErrorDesc string `json:"batchErrorDesc"`

	ErrorCode string `json:"errorCode"`
	ErrorDesc string `json:"errorDesc"`
}

func (e EnvelopeResponseData) CheckError() error {
	if e.ErrorCode != "00" {
		return &Error{
			Code: e.ErrorCode,
			Desc: e.ErrorDesc,
		}
	} else if e.BatchErrorCode != "" {
		return &BatchError{
			Code: e.BatchErrorCode,
			Desc: e.BatchErrorDesc,
		}
	}

	return nil
}

func (s *partnerAPI) Process(ctx context.Context, req Request, result interface{}) error {
	passwordEncrypted, err := s.keyStore.Encrypt(([]byte)(s.password))
	if err != nil {
		return err
	}

	envReq := req.Envelope()
	envReq.SetPassword(passwordEncrypted)
	envReq.SetServiceCode(s.serviceCode)
	envReq.SetUsername(s.username)

	if data := req.Data(); data != nil {
		buf := bytes.NewBuffer(nil)
		if err = MarshalGzipJSON(buf, data); err != nil {
			return err
		}
		envReq.SetData(buf.Bytes())
	}

	envReqJSON, err := json.Marshal(envReq)
	if err != nil {
		return err
	}

	signature, err := s.keyStore.Sign(envReqJSON)
	if err != nil {
		return err
	}

	res, err := s.call(ctx, &Process{
		Cmd:       req.Command(),
		Data:      string(envReqJSON),
		Signature: base64.StdEncoding.EncodeToString(signature),
	})
	if err != nil {
		return err
	}

	var envRes EnvelopeResponse
	err = json.NewDecoder(bytes.NewBufferString(res.Return_)).
		Decode(&envRes)
	if err != nil {
		return err
	}
	if err = s.keyStore.Verify(envRes.Data, envRes.Signature); err != nil {
		return err
	}

	var envResData EnvelopeResponseData
	if err = json.Unmarshal(envRes.Data, &envResData); err != nil {
		return err
	}

	if data := envResData.Data; data != nil {
		// NOTE: VTP also return data in case errors happen.
		// So, we unmarshal data first then check error late.
		err = UnmarshalGzipJSON(bytes.NewReader(envResData.Data), result)
		if err != nil {
			return err
		}
	}

	return envResData.CheckError()
}
