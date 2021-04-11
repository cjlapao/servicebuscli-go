package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/cjlapao/common-go/helper"
	"github.com/cjlapao/common-go/log"
	"github.com/cjlapao/servicebus-go/help"
	"github.com/cjlapao/servicebus-go/servicebus"
	"github.com/cjlapao/servicebus-go/startup"
	"github.com/rs/xid"
)

var logger = log.Get()

func main() {
	logger.Command("************************************************************")
	logger.Command("*      Ivanti Cloud Development Service Bus Tool v1.0      *")
	logger.Command("*                                                          *")
	logger.Command("*  Author: Carlos Lapao                                    *")
	logger.Command("************************************************************")
	connStr := os.Getenv("SERVICEBUS_CONNECTION_STRING")

	helpArg := helper.GetFlagSwitch("help", false)

	module := GetModuleArgument()
	if module == "" {
		help.PrintMainCommandHelper()
		os.Exit(0)
	}

	if connStr == "" {
		help.PrintMissingServiceBusConnectionHelper()
		os.Exit(1)
	}

	switch module {
	case "topic":
		command := GetCommandArgument()
		if command == "" {
			help.PrintTopicMainCommandHelper()
			os.Exit(0)
		}
		switch strings.ToLower(command) {
		case "subscribe":
			if helpArg {
				help.PrintTopicSubscribeCommandHelper()
				os.Exit(0)
			}
			topics := helper.GetFlagArrayValue("topic")
			subscription := helper.GetFlagValue("subscription", "")
			wiretap := helper.GetFlagSwitch("wiretap", false)
			peek := helper.GetFlagSwitch("peek", false)
			if len(topics) == 0 {
				logger.Error("Missing topic name mandatory argument --topic")
				help.PrintTopicSubscribeCommandHelper()
				os.Exit(0)
			}
			if subscription == "" && !wiretap {
				logger.Error("Missing subscription name mandatory argument --subscription")
				help.PrintTopicSubscribeCommandHelper()
				os.Exit(0)
			}

			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, os.Kill)

			var wg sync.WaitGroup
			wg.Add(len(topics))
			var topicSbClients []*servicebus.servicebus
			for _, topic := range topics {
				go func(topicName string) {
					sbcli := servicebus.NewCli(connStr)
					sbcli.UseWiretap = wiretap
					sbcli.Peek = peek

					if sbcli.UseWiretap {
						subscription = "wiretap"
					}

					topicSbClients = append(topicSbClients, sbcli)
					sbcli.SubscribeToTopic(topicName, subscription)
					defer wg.Done()
				}(topic)
			}
			logger.LogHighlight("Use %v to close connection", log.Info, "ctrl+c")
			<-signalChan
			for _, topicCli := range topicSbClients {
				topicCli.CloseTopicListener <- true
			}
			wg.Wait()
			logger.Info("Bye!!!")
			os.Exit(0)
		case "list":
			sbcli := servicebus.NewCli(connStr)
			topics, err := sbcli.ListTopics()
			if err != nil {
				os.Exit(1)
			}
			if len(topics) > 0 {
				logger.Info("Topics:")
				for _, topic := range topics {
					logger.LogHighlight("Topics: %v (last updated at: %v)", log.Info, topic.Name, topic.UpdatedAt.String())
				}
			} else {
				logger.LogHighlight("No topics found  in service bus %v", log.Info, sbcli.Namespace.Name)
			}
		case "list-subscriptions":
			if helpArg {
				help.PrintTopicListSubscriptionsCommandHelper()
				os.Exit(0)
			}
			topic := helper.GetFlagValue("name", "")
			if topic == "" {
				logger.LogHighlight("Missing topic name, use %v=example.topic", log.Error, "--topic")
				help.PrintTopicListSubscriptionsCommandHelper()
				os.Exit(0)
			}

			sbcli := servicebus.NewCli(connStr)
			subscriptions, err := sbcli.ListSubscriptions(topic)
			if err != nil {
				os.Exit(1)
			}

			if len(subscriptions) > 0 {
				logger.Info("Subscriptions:")
				for _, subscription := range subscriptions {
					name := subscription.Name
					if name == "wiretap" {
						name = name
					}
					forwardTo := ""
					activeMsg := "0"
					deadletterMsg := "0"
					scheduledMsg := "0"
					activeMessageCount := *subscription.CountDetails.ActiveMessageCount
					deadletterMessageCount := *subscription.CountDetails.DeadLetterMessageCount
					scheduledMessageCount := *subscription.CountDetails.ScheduledMessageCount

					if activeMessageCount > 0 {
						activeMsg = fmt.Sprint(activeMessageCount)
					}
					if deadletterMessageCount > 0 {
						deadletterMsg = fmt.Sprint(deadletterMessageCount)
					}
					if scheduledMessageCount > 0 {
						scheduledMsg = fmt.Sprint(scheduledMessageCount)
					}
					if subscription.ForwardTo != nil {
						forwardTo = "forwarding to -> " + *subscription.ForwardTo
					}
					logger.LogHighlight("Subscription: %v (messages: %v, dead letters: %v, scheduled: %v) %v", log.Info, name, activeMsg, deadletterMsg, scheduledMsg, forwardTo)
				}
			} else {
				logger.LogHighlight("No subscriptions found on topic %v in service bus %v", log.Info, topic, sbcli.Namespace.Name)
			}
		case "delete":
			if helpArg {
				help.PrintTopicDeleteTopicCommandHelper()
				os.Exit(0)
			}
			topic := helper.GetFlagValue("name", "")
			if topic == "" {
				logger.Error("Missing topic name mandatory argument --name")
				help.PrintTopicDeleteTopicCommandHelper()
				os.Exit(0)
			}
			sbcli := servicebus.NewCli(connStr)
			err := sbcli.DeleteTopic(topic)
			if err != nil {
				os.Exit(1)
			}
		case "create":
			if helpArg {
				help.PrintTopicCreateTopicCommandHelper()
				os.Exit(0)
			}
			topic := helper.GetFlagValue("name", "")
			if topic == "" {
				logger.Error("Missing topic name mandatory argument --name")
				help.PrintTopicCreateTopicCommandHelper()
				os.Exit(0)
			}
			sbcli := servicebus.NewCli(connStr)
			err := sbcli.CreateTopic(topic)
			if err != nil {
				os.Exit(1)
			}
		case "create-subscription":
			if helpArg {
				help.PrintTopicCreateSubscriptionCommandHelper()
				os.Exit(0)
			}
			topicName := helper.GetFlagValue("name", "")
			subscriptionName := helper.GetFlagValue("subscription", "")
			forwardTo := helper.GetFlagValue("forward-to", "")
			forwardDeadLetterTo := helper.GetFlagValue("forward-deadletter-to", "")
			rules := helper.GetFlagArrayValue("with-rule")
			if topicName == "" {
				logger.Error("Missing topic name mandatory argument --name")
				help.PrintTopicCreateSubscriptionCommandHelper()
				os.Exit(0)
			}
			if subscriptionName == "" {
				logger.Error("Missing subscription name mandatory argument --subscription")
				help.PrintTopicCreateSubscriptionCommandHelper()
				os.Exit(0)
			}
			sbcli := servicebus.NewCli(connStr)

			subscription := servicebus.NewSubscription(topicName, subscriptionName)
			subscription.MapMessageForwardFlag(forwardTo)
			subscription.MapDeadLetterForwardFlag(forwardDeadLetterTo)
			for _, rule := range rules {
				subscription.MapRuleFlag(rule)
			}
			err := sbcli.CreateSubscription(subscription)
			if err != nil {
				os.Exit(1)
			}
		case "delete-subscription":
			if helpArg {
				help.PrintTopicDeleteSubscriptionCommandHelper()
				os.Exit(0)
			}
			topic := helper.GetFlagValue("name", "")
			subscription := helper.GetFlagValue("subscription", "")
			if topic == "" {
				logger.Error("Missing topic name mandatory argument --name")
				help.PrintTopicDeleteSubscriptionCommandHelper()
				os.Exit(0)
			}
			if subscription == "" {
				logger.Error("Missing subscription name mandatory argument --subscription")
				help.PrintTopicDeleteSubscriptionCommandHelper()
				os.Exit(0)
			}
			sbcli := servicebus.NewCli(connStr)
			err := sbcli.DeleteSubscription(topic, subscription)
			if err != nil {
				os.Exit(1)
			}
		case "send":
			if helpArg {
				help.PrintTopicSendCommandHelper()
				os.Exit(0)
			}
			topic := helper.GetFlagValue("topic", "")
			tenantID := helper.GetFlagValue("tenant", "11111111-1111-1111-1111-555555550001")
			useDefault := helper.GetFlagSwitch("default", false)
			unoFormat := helper.GetFlagSwitch("uno", false)
			body := helper.GetFlagValue("body", "")
			label := helper.GetFlagValue("label", "ServiceBus.Tools")
			name := helper.GetFlagValue("name", "")
			domain := helper.GetFlagValue("domain", "")
			sender := helper.GetFlagValue("sender", "ServiceBus.Tools")
			version := helper.GetFlagValue("version", "1.0")
			propertiesFlags := helper.GetFlagArrayValue("property")

			if topic == "" {
				logger.Error("Missing topic name mandatory argument --name")
				help.PrintTopicSendCommandHelper()
				os.Exit(0)
			}

			var message map[string]interface{}
			if useDefault && body == "" {
				if !unoFormat {
					domain = "TimeService"
					name = "TimePassed"
				} else {
					domain = ""
					name = ""
				}
				message = map[string]interface{}{
					"Timestamp": time.Now().Format("2006-01-02T15:04:05.00000000-07:00"),
					"TheTime":   time.Now().Format("2006-01-02T15:04:05"),
				}
			} else if body != "" {
				err := json.Unmarshal([]byte(body), &message)
				if err != nil {
					logger.Error(err.Error())
					os.Exit(1)
				}
			} else {
				logger.LogHighlight("Missing message body, use %v='{\"example\": \"object\"}' or use the %v flag, this will generate a TimeService sample message", log.Info, "--body", "--default")
				help.PrintTopicSendCommandHelper()
				os.Exit(0)
			}
			var properties map[string]interface{}
			if len(propertiesFlags) == 0 || useDefault {
				if domain != "" && name != "" {
					label = domain + "." + name
					diagnosticID := xid.New().String()
					properties = map[string]interface{}{
						"X-MsgTypeVersion": version,
						"X-MsgDomain":      domain,
						"X-MsgName":        name,
						"X-Sender":         sender,
						"X-TenantId":       tenantID,
						"Diagnostic-Id":    diagnosticID,
					}
				} else {
					label = topic
					properties = map[string]interface{}{
						"Serialization": "1",
						"TenantId":      tenantID,
					}
				}
			}
			if len(propertiesFlags) > 0 {
				if properties == nil {
					properties = make(map[string]interface{})
				}
				for _, property := range propertiesFlags {
					key, value := helper.MapFlagValue(property)
					if key != "" && value != "" {
						properties[key] = value
					}
				}
			}

			sbcli := servicebus.NewCli(connStr)
			sbcli.SendTopicMessage(topic, message, label, properties)
		default:
			logger.LogHighlight("Invalid command argument %v, please choose a valid argument", log.Info, command)
			help.PrintTopicMainCommandHelper()
		}
		os.Exit(0)
	case "queue":
		command := GetCommandArgument()
		if command == "" {
			help.PrintQueueMainCommandHelper()
			os.Exit(0)
		}
		switch strings.ToLower(command) {
		case "subscribe":
			if helpArg {
				help.PrintQueueSubscribeCommandHelper()
				os.Exit(0)
			}
			queues := helper.GetFlagArrayValue("queue")
			peek := helper.GetFlagSwitch("peek", false)
			if len(queues) == 0 {
				logger.Error("Missing queue name mandatory argument --queue")
				help.PrintQueueSubscribeCommandHelper()
				os.Exit(0)
			}

			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, os.Kill)

			var wg sync.WaitGroup
			wg.Add(len(queues))
			var queueSbClients []*servicebus.servicebus
			for _, queue := range queues {
				go func(queueName string) {
					sbcli := servicebus.NewCli(connStr)
					sbcli.Peek = peek
					queueSbClients = append(queueSbClients, sbcli)
					sbcli.SubscribeToQueue(queueName)
					defer wg.Done()
				}(queue)
			}
			logger.LogHighlight("Use %v to close connection", log.Info, "ctrl+c")
			<-signalChan
			for _, queueCli := range queueSbClients {
				queueCli.CloseQueueListener <- true
			}
			wg.Wait()
			logger.Info("Bye!!!")
			os.Exit(0)
		case "list":
			sbcli := servicebus.NewCli(connStr)
			queues, err := sbcli.ListQueues()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			if len(queues) > 0 {
				logger.Info("Queues:")
				for _, queue := range queues {
					name := queue.Name
					forwardTo := ""
					activeMsg := "0"
					deadletterMsg := "0"
					scheduledMsg := "0"
					activeMessageCount := *queue.CountDetails.ActiveMessageCount
					deadletterMessageCount := *queue.CountDetails.DeadLetterMessageCount
					scheduledMessageCount := *queue.CountDetails.ScheduledMessageCount

					if activeMessageCount > 0 {
						activeMsg = fmt.Sprint(activeMessageCount)
					}
					if deadletterMessageCount > 0 {
						deadletterMsg = fmt.Sprint(deadletterMessageCount)
					}
					if scheduledMessageCount > 0 {
						scheduledMsg = fmt.Sprint(scheduledMessageCount)
					}
					if queue.ForwardTo != nil && strings.TrimSpace(*queue.ForwardTo) != "" {
						forwardTo = "forwarding to -> " + strings.TrimSpace(*queue.ForwardTo)
					}
					logger.LogHighlight("Queue: %v (messages: %v, dead letters: %v, scheduled: %v) %v", log.Info, name, activeMsg, deadletterMsg, scheduledMsg, forwardTo)
				}
			} else {
				logger.LogHighlight("No Queues found in service bus %v", log.Info, sbcli.Namespace.Name)
			}
		case "delete":
			if helpArg {
				help.PrintQueueDeleteCommandHelper()
				os.Exit(0)
			}
			queue := helper.GetFlagValue("name", "")
			if queue == "" {
				logger.Error("Missing queue name mandatory argument --name")
				help.PrintQueueDeleteCommandHelper()
				os.Exit(0)
			}
			sbcli := servicebus.NewCli(connStr)
			err := sbcli.DeleteQueue(queue)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		case "create":
			if helpArg {
				help.PrintQueueCreateCommandHelper()
				os.Exit(0)
			}
			queueName := helper.GetFlagValue("name", "")
			forwardTo := helper.GetFlagValue("forward-to", "")
			forwardDeadLetterTo := helper.GetFlagValue("forward-deadletter-to", "")
			if queueName == "" {
				logger.Error("Missing queue name mandatory argument --name")
				help.PrintQueueCreateCommandHelper()
				os.Exit(0)
			}

			sbcli := servicebus.NewCli(connStr)

			queue := servicebus.NewQueue(queueName)
			queue.MapMessageForwardFlag(forwardTo)
			queue.MapDeadLetterForwardFlag(forwardDeadLetterTo)

			err := sbcli.CreateQueue(queue)
			if err != nil {
				os.Exit(1)
			}
		case "send":
			if helpArg {
				help.PrintQueueSendCommandHelper()
				os.Exit(0)
			}
			queue := helper.GetFlagValue("queue", "")
			tenantID := helper.GetFlagValue("tenant", "11111111-1111-1111-1111-555555550001")
			useDefault := helper.GetFlagSwitch("default", false)
			unoFormat := helper.GetFlagSwitch("uno", false)
			body := helper.GetFlagValue("body", "")
			label := helper.GetFlagValue("label", "ServiceBus.Tools")
			name := helper.GetFlagValue("name", "")
			domain := helper.GetFlagValue("domain", "")
			sender := helper.GetFlagValue("sender", "ServiceBus.Tools")
			version := helper.GetFlagValue("version", "1.0")
			propertiesFlags := helper.GetFlagArrayValue("property")

			if queue == "" {
				logger.Error("Missing queue name mandatory argument --name")
				help.PrintQueueSendCommandHelper()
				os.Exit(0)
			}

			var message map[string]interface{}
			if useDefault && body == "" {
				if !unoFormat {
					domain = "TimeService"
					name = "TimePassed"
				} else {
					domain = ""
					name = ""
				}
				message = map[string]interface{}{
					"Timestamp": time.Now().Format("2006-01-02T15:04:05.00000000-07:00"),
					"TheTime":   time.Now().Format("2006-01-02T15:04:05"),
				}
			} else if body != "" {
				err := json.Unmarshal([]byte(body), &message)
				if err != nil {
					logger.Error(err.Error())
					os.Exit(1)
				}
			} else {
				logger.LogHighlight("Missing message body, use %v='{\"example\": \"object\"}' or use the %v flag, this will generate a TimeService sample message", log.Error, "--body", "--default")
				help.PrintTopicSendCommandHelper()
				os.Exit(0)
			}
			var properties map[string]interface{}
			if len(propertiesFlags) == 0 || useDefault {
				if domain != "" && name != "" {
					label = domain + "." + name
					diagnosticID := xid.New().String()
					properties = map[string]interface{}{
						"X-MsgTypeVersion": version,
						"X-MsgDomain":      domain,
						"X-MsgName":        name,
						"X-Sender":         sender,
						"X-TenantId":       tenantID,
						"Diagnostic-Id":    diagnosticID,
					}
				} else {
					label = queue
					properties = map[string]interface{}{
						"Serialization": "1",
						"TenantId":      tenantID,
					}
				}
			}
			if len(propertiesFlags) > 0 {
				if properties == nil {
					properties = make(map[string]interface{})
				}
				for _, property := range propertiesFlags {
					key, value := helper.MapFlagValue(property)
					if key != "" && value != "" {
						properties[key] = value
					}
				}
			}

			sbcli := servicebus.NewCli(connStr)
			sbcli.SendQueueMessage(queue, message, label, properties)

		default:
			logger.LogHighlight("Invalid command argument %v, please choose a valid argument", log.Error, command)
			help.PrintQueueMainCommandHelper()
		}
		os.Exit(0)
	default:

		help.PrintMainCommandHelper()
	}
	if helpArg {
		help.PrintMainCommandHelper()
		os.Exit(0)
	}
}

func GetModuleArgument() string {
	args := os.Args[1:]

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		help.PrintMainCommandHelper()
		startup.Exit(0)
	}

	return args[0]
}

func GetCommandArgument() string {
	args := os.Args[2:]

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return ""
	}

	return args[0]
}

func GetSubCommandArgument() string {
	args := os.Args[3:]

	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return ""
	}

	return args[0]
}
