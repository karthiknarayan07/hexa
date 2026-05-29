package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go-example/internal/task/domain"
	"go-example/internal/task/ports/inbound"
	"go-example/internal/task/ports/outbound"
)

/*
TaskAPI is an inbound adapter.

Its job is to translate HTTP details into use-case calls and translate the
results back into HTTP responses. It must not contain business rules.
*/
type TaskAPI struct {
	createTask   inbound.CreateTaskUseCase
	startTask    inbound.StartTaskUseCase
	completeTask inbound.CompleteTaskUseCase
	listTasks    inbound.ListTasksUseCase
}

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type errorResponse struct {
	Error string `json:"error"`
}

/*
NewTaskAPI receives the inbound ports needed by this adapter.

The adapter depends only on interfaces from the core boundary,
which keeps the dependency arrow pointing inward.
*/
func NewTaskAPI(
	createTask inbound.CreateTaskUseCase,
	startTask inbound.StartTaskUseCase,
	completeTask inbound.CompleteTaskUseCase,
	listTasks inbound.ListTasksUseCase,
) *TaskAPI {
	return &TaskAPI{
		createTask:   createTask,
		startTask:    startTask,
		completeTask: completeTask,
		listTasks:    listTasks,
	}
}

/*
Routes exposes the HTTP endpoints for the example.

The returned handler is still an adapter artifact. The application core does
not know which URLs or HTTP verbs are used to reach its use cases.
*/
func (api *TaskAPI) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", api.handleCreateTask)
	mux.HandleFunc("GET /tasks", api.handleListTasks)
	mux.HandleFunc("POST /tasks/{id}/start", api.handleStartTask)
	mux.HandleFunc("POST /tasks/{id}/complete", api.handleCompleteTask)

	return mux
}

/*
handleCreateTask converts an HTTP request body into a create-task command.
*/
func (api *TaskAPI) handleCreateTask(writer http.ResponseWriter, request *http.Request) {
	var payload createTaskRequest

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		api.writeError(writer, http.StatusBadRequest, fmt.Errorf("decode request body: %w", err))
		return
	}

	task, err := api.createTask.CreateTask(request.Context(), inbound.CreateTaskCommand{
		Title:       payload.Title,
		Description: payload.Description,
	})
	if err != nil {
		api.writeDomainAwareError(writer, err)
		return
	}

	api.writeJSON(writer, http.StatusCreated, task)
}

/*
handleListTasks delegates the query to the inbound use-case port.
*/
func (api *TaskAPI) handleListTasks(writer http.ResponseWriter, request *http.Request) {
	tasks, err := api.listTasks.ListTasks(request.Context())
	if err != nil {
		api.writeDomainAwareError(writer, err)
		return
	}

	api.writeJSON(writer, http.StatusOK, tasks)
}

/*
handleStartTask resolves the route parameter and forwards the call inward.
*/
func (api *TaskAPI) handleStartTask(writer http.ResponseWriter, request *http.Request) {
	taskID := request.PathValue("id")

	task, err := api.startTask.StartTask(request.Context(), taskID)
	if err != nil {
		api.writeDomainAwareError(writer, err)
		return
	}

	api.writeJSON(writer, http.StatusOK, task)
}

/*
handleCompleteTask mirrors the same adapter behavior for completion.
*/
func (api *TaskAPI) handleCompleteTask(writer http.ResponseWriter, request *http.Request) {
	taskID := request.PathValue("id")

	task, err := api.completeTask.CompleteTask(request.Context(), taskID)
	if err != nil {
		api.writeDomainAwareError(writer, err)
		return
	}

	api.writeJSON(writer, http.StatusOK, task)
}

/*
writeDomainAwareError maps core errors to transport-level HTTP responses.

This mapping belongs in the adapter because HTTP status codes are transport
concerns, not domain concerns.
*/
func (api *TaskAPI) writeDomainAwareError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, outbound.ErrTaskNotFound):
		api.writeError(writer, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrTaskTitleRequired), errors.Is(err, domain.ErrTaskMustBeStartedFirst):
		api.writeError(writer, http.StatusBadRequest, err)
	case errors.Is(err, domain.ErrTaskAlreadyStarted), errors.Is(err, domain.ErrTaskAlreadyCompleted):
		api.writeError(writer, http.StatusConflict, err)
	case errors.Is(err, domain.ErrInvalidTaskState):
		api.writeError(writer, http.StatusBadRequest, err)
	default:
		api.writeError(writer, http.StatusInternalServerError, err)
	}
}

/*
writeJSON serializes a value as JSON and sends it to the client.
*/
func (api *TaskAPI) writeJSON(writer http.ResponseWriter, statusCode int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)

	if err := json.NewEncoder(writer).Encode(payload); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

/*
writeError keeps error responses in one JSON shape.
*/
func (api *TaskAPI) writeError(writer http.ResponseWriter, statusCode int, err error) {
	api.writeJSON(writer, statusCode, errorResponse{Error: err.Error()})
}
