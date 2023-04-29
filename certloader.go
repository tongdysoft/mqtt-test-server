package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func LoadCert(certPEMByte []byte, keyPEMByte []byte, password string) tls.Certificate {
	// keyPEMBlock, rest := pem.Decode(keyPEMByte)
	// if len(rest) > 0 {
	// 	logPrint("X", fmt.Sprintf("%s%s: %s", lang("SERVERKEY"), lang("ERROR"), rest))
	// 	return tls.Certificate{}
	// }
	if len(password) > 0 { // x509.IsEncryptedPEMBlock(keyPEMBlock)
		// keyDePEMByte, err := x509.DecryptPEMBlock(keyPEMBlock, []byte(password))
		// if err != nil {
		// 	logPrint("X", fmt.Sprintf("%s%s: %s", lang("DECRYPT"), lang("ERROR"), err.Error()))
		// 	return tls.Certificate{}
		// }
		// 取出 RSA 私钥
		key, err := x509.ParsePKCS1PrivateKey(keyPEMByte)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s: %s", lang("ParsePrivateKey"), err.Error()))
			return tls.Certificate{}
		}
		// 整合成新的 PEM
		var keyNewPemByte []byte = pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(key),
			},
		)
		pem, err := tls.X509KeyPair(certPEMByte, keyNewPemByte)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s%s: %s", lang("SERVERCERT"), lang("ERROR"), err.Error()))
		}
		return pem
	} else {
		pem, err := tls.X509KeyPair(certPEMByte, keyPEMByte)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s%s: %s", lang("SERVERCERT"), lang("ERROR"), err.Error()))
		}
		return pem
	}
}
