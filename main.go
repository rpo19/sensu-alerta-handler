package main

import (
	"time"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"net/http"
	"bytes"

	"github.com/sensu/sensu-go/types"
	"github.com/spf13/cobra"
)

type AlertaAttributes struct {
	Occurrences int64	`json:"subscribers"`
	LastOK	string	`json:"lastok"`
	Issued	string	`json:"issued"`
	Executed	string	`json:"executed"`
	LastSeen	string	`json:"lastseen"`
	Output	string	`json:"output"`
}

type AlertaMessage struct {
    Origin	string	`json:"origin"`
    Resource	string	`json:"resource"`
	Event	string	`json:"event"`
	Group	string	`json:"group"`
	Severity	string	`json:"severity"`
	Environment	string	`json:"environment"`
	Service	[]string	`json:"service"`
	Tags	[]string	`json:"tags"`
	Text	string	`json:"text"`
	Summary	string	`json:"summary"`
	Value	string	`json:"value"`
	Type	string	`json:"type"`
	Attributes	AlertaAttributes	`json:"attributes"`
	RawData	string	`json:"rawData"`
}

var (
	possibleEnvironments	[2]string
	endpoint string
	environment	string
	key    string
	timeout    int
	stdin      *os.File
	tags	[]string
	attributes AlertaAttributes
	receivedJSON	string
)

func main() {
	possibleEnvironments = [2]string{"Production", "Development"}
	
	rootCmd := configureRootCommand()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

func configureRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sensu-alerta-handler",
		Short: "The Sensu Go Alerta handler for notifying a channel",
		RunE:  run,
	}

	/*
		Sensitive flags
		default to using envvar value
		do not mark as required
		manually test for empty value
	*/
	cmd.Flags().StringVarP(&endpoint,
		"endpoint",
		"e",
		os.Getenv("ALERTA_ENDPOINT"),
		"The http endpoint of alerta")

	cmd.Flags().StringVarP(&environment,
		"environment",
		"E",
		os.Getenv("ALERTA_ENVIRONMENT"),
		"Alerta environment (Development, Production")

	cmd.Flags().StringVarP(&key,
		"key",
		"k",
		os.Getenv("ALERTA_KEY"),
		"Alerta http auth key")

	cmd.Flags().IntVarP(&timeout,
		"timeout",
		"t",
		10,
		"The amount of seconds to wait before terminating the handler")

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("invalid argument(s) received")
	}
	if endpoint == "" {
		return fmt.Errorf("endpoint is empty")

	}
	if environment == "" {
		environment = possibleEnvironments[0]
	} else if environment != possibleEnvironments[0] && environment != possibleEnvironments[1] {
		return fmt.Errorf(`wrong environment: "%s"`, environment)
	}
	if stdin == nil {
		stdin = os.Stdin
	}

	eventJSON, err := ioutil.ReadAll(stdin)
	receivedJSON = string(eventJSON)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %s", err.Error())
	}

	event := &types.Event{}
	err = json.Unmarshal(eventJSON, event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal stdin data: %s", eventJSON)
	}

	if err = validateEvent(event); err != nil {
		return errors.New(err.Error())
	}

	if err = sendMessage(event); err != nil {
		return errors.New(err.Error())
	}

	return nil
}

func formattedEventAction(event *types.Event) string {
	switch event.Check.Status {
	case 0:
		return "RESOLVED"
	default:
		return "ALERT"
	}
}

func chomp(s string) string {
	return strings.Trim(strings.Trim(strings.Trim(s, "\n"), "\r"), "\r\n")
}

func eventKey(event *types.Event) string {
	return fmt.Sprintf("%s/%s", event.Entity.Name, event.Check.Name)
}

func eventSummary(event *types.Event, maxLength int) string {
	output := chomp(event.Check.Output)
	if len(event.Check.Output) > maxLength {
		output = output[0:maxLength] + "..."
	}
	return fmt.Sprintf("%s:%s", eventKey(event), output)
}

func formattedMessage(event *types.Event) string {
	return fmt.Sprintf("%s - %s", formattedEventAction(event), eventSummary(event, 100))
}

func messageColor(event *types.Event) string {
	switch event.Check.Status {
	case 0:
		return "good"
	case 2:
		return "danger"
	default:
		return "warning"
	}
}

func messageStatus(event *types.Event) string {
	switch event.Check.Status {
	case 0:
		return "normal"
	case 1:
		return "warning"
	case 2:
		return "critical"
	default:
		return "indeterminate"
	}
}

func getAttributes(event *types.Event) AlertaAttributes {
	attr := AlertaAttributes{
		event.Check.Occurrences,
		fmt.Sprintf("%s", time.Unix(event.Check.LastOK, 0)),
		fmt.Sprintf("%s", time.Unix(event.Check.Issued, 0)),
		fmt.Sprintf("%s", time.Unix(event.Check.Executed, 0)),
		fmt.Sprintf("%s", time.Unix(event.Entity.LastSeen, 0)),
		event.Check.Output}
	return attr
}

func alertaPayload(event *types.Event) AlertaMessage {
	for _,handler := range event.Check.Handlers {
		tags = append(tags, fmt.Sprintf("handler=%s", handler))
	}

	attributes = getAttributes(event)

	var message = AlertaMessage {
		Origin:	event.Entity.System.Hostname,
		Resource:	event.Entity.Name,
		Event:	event.Check.Name,
		Group:	"SensuGo",
		Severity:	messageStatus(event),
		Environment:	environment,
		Service:	event.Entity.Subscriptions,
		Tags:	tags,
		Text:	formattedMessage(event),
		Summary:	formattedMessage(event),
		Value:	event.Check.State,
		Type:	"sensuGoAlert",
		Attributes:	attributes,
		RawData:	receivedJSON }

	return message
}

func sendMessage(event *types.Event) error {
	bbs, errJson := json.Marshal(alertaPayload(event))
	if errJson != nil {
		fmt.Println("Error while encoding in json!")
		fmt.Println(errJson)
		return errJson
	}

	encodedMsg := bytes.NewBuffer(bbs)
	req, errReq := http.NewRequest("POST", endpoint, encodedMsg)
	if errReq != nil {
		fmt.Println("Error while creating HTTP Request!")
		fmt.Println(errReq)
		return errReq
	}

	req.Header.Add("Content-Type", "application/json")
	if key != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Key %s", key))
	}
	resp, errDo := http.DefaultClient.Do(req)

	if errDo != nil {
		fmt.Println("Something went wrong while doing HTTP Request!")
		fmt.Println(errDo)
		fmt.Println(resp)
		return errDo
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(fmt.Sprintf("Alerta answered with non 2XX code: %d !\n%+v", resp.StatusCode, resp))
	}

	return nil
}

func validateEvent(event *types.Event) error {
	if event.Timestamp <= 0 {
		return errors.New("timestamp is missing or must be greater than zero")
	}

	if event.Entity == nil {
		return errors.New("entity is missing from event")
	}

	if !event.HasCheck() {
		return errors.New("check is missing from event")
	}

	if err := event.Entity.Validate(); err != nil {
		return err
	}

	if err := event.Check.Validate(); err != nil {
		return errors.New(err.Error())
	}

	return nil
}
