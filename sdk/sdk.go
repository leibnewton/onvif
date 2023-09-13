package sdk

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/juju/errors"
	"github.com/rs/zerolog"
)

var (
	// LoggerContext is the builder of a zerolog.Logger that is exposed to the application so that
	// options at the CLI might alter the formatting and the output of the logs.
	LoggerContext = zerolog.
			New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp()

	// Logger is a zerolog logger, that can be safely used from any part of the application.
	// It gathers the format and the output.
	Logger = LoggerContext.Logger()
)

// sample xml:
// <s:Fault>
//   <s:Code><s:Value>s:Sender</s:Value><s:Subcode><s:Value>ter:NotAuthorized</s:Value></s:Subcode></s:Code>
//   <s:Reason><s:Text xml:lang="en">Sender not Authorized. Invalid username or password!</s:Text></s:Reason>
// </s:Fault>
type Fault struct {
	StatusCode int `xml:"-"`

	Code    string `xml:">Value"`
	Subcode string `xml:"Code>Subcode>Value"` // NotAuthorized,ActionNotSupported,...
	Reason  string `xml:">Text"`
}

func (f Fault) Error() string {
	return fmt.Sprintf("http-status: %d, code: %s/%s, detail: %s", f.StatusCode, f.Code, f.Subcode, f.Reason)
}

func ReadAndParse(ctx context.Context, httpReply *http.Response, reply interface{}, tag string) error {
	Logger.Debug().
		Str("msg", httpReply.Status).
		Int("status", httpReply.StatusCode).
		Str("action", tag).
		Msg("RPC")
	// TODO(jfsmig): extract the deadline from ctx.Deadline() and apply it on the reply reading
	b, err := ioutil.ReadAll(httpReply.Body)
	if err != nil {
		return errors.Annotate(err, "read")
	}

	httpReply.Body.Close()

	type Envelope struct {
		Fault Fault `xml:"Body>Fault"`
	}
	var envFault Envelope
	envFault.Fault.StatusCode = httpReply.StatusCode
	err = xml.Unmarshal(b, &envFault)
	if err != nil {
		return errors.Annotate(err, "decode fault info")
	} else if envFault.Fault.StatusCode != 200 || len(envFault.Fault.Code) > 0 {
		return &envFault.Fault
	}

	err = xml.Unmarshal(b, reply)
	return errors.Annotate(err, "decode")
}
