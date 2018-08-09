package gojenkins

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

type Credentials struct {
	Raw     *CredentialsResponse
	Jenkins *Jenkins
	Base    string
}

type UserCredential struct {
	Description string `xml:"description"`
	DisplayName string `xml:"displayName"`
	Fingerprint string `xml:"fingerprint"`
	FullName    string `xml:"fullName"`
	ID          string `xml:"id"`
	TypeName    string `xml:"typeName"`
}

type DomainWrapper struct {
	XMLName         xml.Name         `xml:"domainWrapper"`
	Class           string           `xml:"_class,attr"`
	Description     string           `xml:"description"`
	DisplayName     string           `xml:"displayName"`
	FullDisplayName string           `xml:"fullDisplayName"`
	FullName        string           `xml:"fullName"`
	Global          string           `xml:"global"`
	URLName         string           `xml:"urlName"`
	UserCredentials []UserCredential `xml:"credential"`
}

type CredentialsResponse struct {
}

func (c Credentials) Create(credentialsData string) error {
	var data string
	_, err := c.Jenkins.Requester.Post(c.Base+"createCredentials", bytes.NewBufferString(credentialsData), &data, nil)
	if err != nil {
		fmt.Println("Credentials Create:", err)
	}
	return err
}

func (c Credentials) GetAll() ([]UserCredential, error) {
	var data string
	endpoint := c.Base + "api/xml"
	qeuryString := map[string]string{
		"depth": "1",
	}
	_, err := c.Jenkins.Requester.GetXML(endpoint, &data, qeuryString)
	if err != nil {
		return nil, err
	}
	var result DomainWrapper
	err = xml.Unmarshal(bytes.NewBufferString(data).Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return result.UserCredentials, nil
}

// Remove /credentials/store/system/domain/_/credential/auto-test-888/doDelete
func (c Credentials) Remove(credentialsID string) error {
	var data string
	endpoint := fmt.Sprintf("%scredential/%s/doDelete", c.Base, credentialsID)
	_, err := c.Jenkins.Requester.Post(endpoint, nil, &data, nil)
	if err != nil {
		fmt.Println("Credentials Remove:", err)
	}
	return err
}
