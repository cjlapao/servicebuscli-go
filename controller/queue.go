package controller

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/cjlapao/servicebuscli-go/entities"
	"github.com/gorilla/mux"
)

// GetQueues Gets all queues in the current namespace
func (c *Controller) GetQueues(w http.ResponseWriter, r *http.Request) {
	errorResponse := entities.ApiErrorResponse{}
	azQueues, err := sbcli.ListQueues()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	queues := make([]entities.QueueResponse, 0)

	if len(azQueues) == 0 {
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(queues)
		return
	}

	for _, azQueue := range azQueues {
		queue := entities.QueueResponse{}
		queue.FromServiceBus(azQueue)
		queues = append(queues, queue)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(queues)
}

// GetQueue Gets the details of a specific Queue in the current namespace
func (c *Controller) GetQueue(w http.ResponseWriter, r *http.Request) {
	errorResponse := entities.ApiErrorResponse{}
	vars := mux.Vars(r)
	queueName := vars["queueName"]

	// Checking for null parameters
	if queueName == "" {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = "Queue name parameter cannot be null or empty"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	queue, err := sbcli.GetQueueDetails(queueName)

	if queue == nil && strings.Contains(err.Error(), "not found") {
		w.WriteHeader(http.StatusNotFound)
		errorResponse.Code = http.StatusNotFound
		errorResponse.Error = "Queue Not Found"
		errorResponse.Message = "Queue with name " + queueName + " was not found in " + sbcli.Namespace.Name
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	if queue == nil && err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	response := entities.QueueResponse{}
	response.FromServiceBus(queue)
	json.NewEncoder(w).Encode(response)
}

// UpsertQueue Update or Insert a Queue in the current namespace
func (c *Controller) UpsertQueue(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	errorResponse := entities.ApiErrorResponse{}
	upsert := false
	if r.Method == "PUT" {
		upsert = true
	}

	// Body cannot be null error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Empty Body"
		errorResponse.Message = "The body of the request is null or empty"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	queueRequest := entities.QueueRequest{}
	err = json.Unmarshal(reqBody, &queueRequest)

	// Body deserialization error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Failed Body Deserialization"
		errorResponse.Message = "There was an error deserializing the body of the request"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	isValid, validError := queueRequest.IsValid()

	if !isValid {
		if validError != nil {
			w.WriteHeader(int(validError.Code))
			json.NewEncoder(w).Encode(validError)
			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}

	if !upsert {
		queueExists, _ := sbcli.GetQueue(queueRequest.Name)

		if queueExists != nil {
			w.WriteHeader(http.StatusBadRequest)
			found := entities.ApiSuccessResponse{
				Message: "The Queue " + queueRequest.Name + " already exists in " + sbcli.Namespace.Name + ", ignoring",
			}
			json.NewEncoder(w).Encode(found)
			return
		}
	}

	err = sbcli.CreateQueue(queueRequest)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Error Creating Queue"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	createdQueue, err := sbcli.GetQueueDetails(queueRequest.Name)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Error Creating Queue"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	response := entities.QueueResponse{}
	response.FromServiceBus(createdQueue)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// DeleteQueue Deletes a Queue in the namespace
func (c *Controller) DeleteQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queueName"]
	errorResponse := entities.ApiErrorResponse{}

	if queueName == "" {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = "Queue name parameter cannot be null or empty"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	queue, err := sbcli.GetQueueDetails(queueName)
	if queue == nil && strings.Contains(err.Error(), "not found") {
		w.WriteHeader(http.StatusNotFound)
		errorResponse.Code = http.StatusNotFound
		errorResponse.Error = "Queue Not Found"
		errorResponse.Message = "Queue with name " + queueName + " was not found in " + sbcli.Namespace.Name
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	if queue == nil && err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	err = sbcli.DeleteQueue(queueName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		errorResponse.Code = http.StatusNotFound
		errorResponse.Error = "Server Error"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// SendQueueMessage Sends a Message to a Queue in the current namespace
func (c *Controller) SendQueueMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queueName"]
	reqBody, err := ioutil.ReadAll(r.Body)
	errorResponse := entities.ApiErrorResponse{}

	if queueName == "" {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Server Error"
		errorResponse.Message = "Queue name parameter cannot be null or empty"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// Body cannot be nil error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Empty Body"
		errorResponse.Message = "The body of the request is null or empty"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	message := entities.MessageRequest{}
	err = json.Unmarshal(reqBody, &message)

	// Body deserialization error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Failed Body Deserialization"
		errorResponse.Message = "There was an error deserializing the body of the request"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	sbMessage, err := message.ToServiceBus()

	// Convert to ServiceBus Message error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Failed Conversion"
		errorResponse.Message = "There was an error converting the request to a service bus message"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	err = sbcli.SendQueueServiceBusMessage(queueName, sbMessage)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Error Sending Queue Message"
		errorResponse.Message = "There was an error sending message to queue " + queueName
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	response := entities.ApiSuccessResponse{
		Message: "Message " + message.Label + " was sent successfully to " + queueName + " queue",
		Data:    message.Data,
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

// GetQueueMessages Gets messages from a Queue in the current namespace
func (c *Controller) GetQueueMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queueName"]
	queryValues := r.URL.Query()
	qtyValue := queryValues.Get("qty")
	peekValue := queryValues.Get("peek")
	if qtyValue == "" {
		qtyValue = "0"
	}

	qty, qtyErr := strconv.Atoi(qtyValue)
	if qtyErr != nil {
		qty = 0
	}

	peek := false
	if peekValue == "true" {
		peek = true
	}

	errorResponse := entities.ApiErrorResponse{}

	// Queue Name cannot be nil
	if queueName == "" {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Queue name is null"
		errorResponse.Message = "Queue name cannot be null"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	result, err := sbcli.GetQueueActiveMessages(queueName, qty, peek)

	// Body deserialization error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Failed Message Data Deserialization"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	if len(result) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		response := make([]entities.MessageResponse, 0)
		for _, msg := range result {
			entityMsg := entities.MessageResponse{}
			entityMsg.FromServiceBus(&msg)
			response = append(response, entityMsg)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}
}

// GetQueueDeadLetterMessages Gets dead letters from a Queue in the current namespace
func (c *Controller) GetQueueDeadLetterMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queueName"]
	queryValues := r.URL.Query()
	qtyValue := queryValues.Get("qty")
	peekValue := queryValues.Get("peek")
	if qtyValue == "" {
		qtyValue = "0"
	}

	qty, qtyErr := strconv.Atoi(qtyValue)
	if qtyErr != nil {
		qty = 0
	}

	peek := false
	if peekValue == "true" {
		peek = true
	}

	errorResponse := entities.ApiErrorResponse{}

	// Queue Name cannot be nil
	if queueName == "" {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Queue name is null"
		errorResponse.Message = "Queue name cannot be null"
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	result, err := sbcli.GetQueueDeadLetterMessages(queueName, qty, peek)

	// Body deserialization error
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Error = "Failed Message Data Deserialization"
		errorResponse.Message = err.Error()
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	if len(result) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		response := make([]entities.MessageResponse, 0)
		for _, msg := range result {
			entityMsg := entities.MessageResponse{}
			entityMsg.FromServiceBus(&msg)
			response = append(response, entityMsg)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}
}
