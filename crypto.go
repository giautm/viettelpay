package viettelpay

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
)

type KeyStore interface {
	Sign(data []byte) (signature []byte, err error)
	Verify(data, signature []byte) (err error)
	Decrypt(msg []byte) (string, error)
	Encrypt(msg []byte) (string, error)
}

type keyStore struct {
	partnerPrivateKey *rsa.PrivateKey
	viettelPublicKey  *rsa.PublicKey
}

func NewKeyStore(partnerPriKey, viettelPubKey []byte) (_ KeyStore, err error) {
	keys := &keyStore{}

	if keys.partnerPrivateKey, err = x509.ParsePKCS1PrivateKey(partnerPriKey); err != nil {
		return nil, err
	}

	if key, err := x509.ParsePKIXPublicKey(viettelPubKey); err != nil {
		return nil, err
	} else if rsaKey, ok := key.(*rsa.PublicKey); ok {
		keys.viettelPublicKey = rsaKey
	} else {
		return nil, errors.New("invalid key type")
	}

	return keys, nil
}

func (s *keyStore) Sign(data []byte) ([]byte, error) {
	hashed := sha1.Sum(data)
	return s.partnerPrivateKey.Sign(rand.Reader, hashed[:], crypto.SHA1)
}

func (s *keyStore) Verify(data, signature []byte) error {
	hashed := sha1.Sum(data)
	return rsa.VerifyPKCS1v15(s.viettelPublicKey, crypto.SHA1, hashed[:], signature)
}

func (s *keyStore) Decrypt(msg []byte) (string, error) {
	buf := bytes.NewBuffer(nil)
	err := Decrypt(buf, bytes.NewReader(msg), len(msg), s.partnerPrivateKey)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *keyStore) Encrypt(msg []byte) (string, error) {
	buf := bytes.NewBuffer(nil)
	err := Encrypt(buf, bytes.NewReader(msg), len(msg), s.viettelPublicKey)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GenerateKeysPEM(prvKeyDst, pubKeyDst io.Writer, bits int) error {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}

	err = pem.Encode(prvKeyDst, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err != nil {
		return err
	}

	asn1Bytes, err := asn1.Marshal(key.PublicKey)
	if err != nil {
		return err
	}

	err = pem.Encode(pubKeyDst, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	})
	return err
}

func Decrypt(dst io.Writer, src io.Reader, srcSize int, privateKey *rsa.PrivateKey) error {
	keySize := privateKey.Size()
	base64BlockSize := base64.StdEncoding.EncodedLen(keySize)
	iterations := srcSize / base64BlockSize

	ciphertext := make([]byte, base64.StdEncoding.DecodedLen(base64BlockSize))
	r := base64.NewDecoder(base64.StdEncoding, src)
	for i := 0; i < iterations; i++ {
		n, err := r.Read(ciphertext)
		if err != nil {
			return err
		}

		reverseBytes(ciphertext[:n])
		plaintext, err := privateKey.Decrypt(rand.Reader, ciphertext[:n], nil)
		if err != nil {
			return err
		}

		dst.Write(plaintext)
	}

	return nil
}

func Encrypt(dst io.Writer, src io.Reader, srcSize int, publicKey *rsa.PublicKey) error {
	b64 := base64.NewEncoder(base64.StdEncoding, dst)

	keySize := publicKey.Size()
	maxLength := keySize - 42
	iterations := srcSize / maxLength

	plaintext := make([]byte, maxLength)
	for i := 0; i <= iterations; i++ {
		n, err := src.Read(plaintext)
		if err != nil {
			return err
		}

		ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, plaintext[:n])
		if err != nil {
			return err
		}

		reverseBytes(ciphertext)
		b64.Write(ciphertext)

		// Flush base64 chunk
		if err = b64.Close(); err != nil {
			return err
		}
	}

	return nil
}

func reverseBytes(in []byte) []byte {
	for l, r := 0, len(in)-1; l < r; {
		in[l], in[r] = in[r], in[l]
		l++
		r--
	}

	return in
}
