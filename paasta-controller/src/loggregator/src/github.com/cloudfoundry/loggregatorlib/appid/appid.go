package appid

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gogo/protobuf/proto"
)

func FromUrl(u *url.URL) string {
	appId := u.Query().Get("app")
	return appId
}

func FromProtobufferMessage(data []byte) (appId string, err error) {
	receivedMessage := &logmessage.LogMessage{}
	err = proto.Unmarshal(data, receivedMessage)
	if err == nil {
		return *receivedMessage.AppId, nil
	}

	receivedEnvelope := &logmessage.LogEnvelope{}
	err = proto.Unmarshal(data, receivedEnvelope)
	if err != nil {
		err = errors.New(fmt.Sprintf("Log data could not be unmarshaled to message or envelope. Dropping it... Error: %v. Data: %v", err, data))
		return "", err
	}
	return *receivedEnvelope.RoutingKey, nil
}
