// Code generated by protoc-gen-go-http. DO NOT EDIT.
// versions:
// - protoc-gen-go-http v2.8.4
// - protoc             v3.12.4
// source: nydusify/v1/nydusify.proto

package v1

import (
	context "context"
	http "github.com/go-kratos/kratos/v2/transport/http"
	binding "github.com/go-kratos/kratos/v2/transport/http/binding"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the kratos package it is being compiled against.
var _ = new(context.Context)
var _ = binding.EncodeURL

const _ = http.SupportPackageIsVersion1

const OperationNydusifyServiceCancelTask = "/nydusify.v1.NydusifyService/CancelTask"
const OperationNydusifyServiceGetTaskStatus = "/nydusify.v1.NydusifyService/GetTaskStatus"
const OperationNydusifyServiceListTasks = "/nydusify.v1.NydusifyService/ListTasks"
const OperationNydusifyServiceRetryCallback = "/nydusify.v1.NydusifyService/RetryCallback"
const OperationNydusifyServiceSubmitCommitTask = "/nydusify.v1.NydusifyService/SubmitCommitTask"

type NydusifyServiceHTTPServer interface {
	// CancelTask Cancel a running task
	CancelTask(context.Context, *CancelTaskRequest) (*CancelTaskResponse, error)
	// GetTaskStatus Get task execution status and results
	GetTaskStatus(context.Context, *GetTaskStatusRequest) (*GetTaskStatusResponse, error)
	// ListTasks List all tasks with optional filtering
	ListTasks(context.Context, *ListTasksRequest) (*ListTasksResponse, error)
	// RetryCallback Retry callback notification for a completed task
	RetryCallback(context.Context, *RetryCallbackRequest) (*RetryCallbackResponse, error)
	// SubmitCommitTask Submit nydusify commit task for async execution
	SubmitCommitTask(context.Context, *SubmitCommitTaskRequest) (*SubmitCommitTaskResponse, error)
}

func RegisterNydusifyServiceHTTPServer(s *http.Server, srv NydusifyServiceHTTPServer) {
	r := s.Route("/")
	r.POST("/v1/nydusify/commit", _NydusifyService_SubmitCommitTask0_HTTP_Handler(srv))
	r.GET("/v1/nydusify/tasks/{task_id}", _NydusifyService_GetTaskStatus0_HTTP_Handler(srv))
	r.GET("/v1/nydusify/tasks", _NydusifyService_ListTasks0_HTTP_Handler(srv))
	r.DELETE("/v1/nydusify/tasks/{task_id}", _NydusifyService_CancelTask0_HTTP_Handler(srv))
	r.POST("/v1/nydusify/tasks/{task_id}/retry-callback", _NydusifyService_RetryCallback0_HTTP_Handler(srv))
}

func _NydusifyService_SubmitCommitTask0_HTTP_Handler(srv NydusifyServiceHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in SubmitCommitTaskRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationNydusifyServiceSubmitCommitTask)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.SubmitCommitTask(ctx, req.(*SubmitCommitTaskRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*SubmitCommitTaskResponse)
		return ctx.Result(200, reply)
	}
}

func _NydusifyService_GetTaskStatus0_HTTP_Handler(srv NydusifyServiceHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in GetTaskStatusRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		if err := ctx.BindVars(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationNydusifyServiceGetTaskStatus)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.GetTaskStatus(ctx, req.(*GetTaskStatusRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*GetTaskStatusResponse)
		return ctx.Result(200, reply)
	}
}

func _NydusifyService_ListTasks0_HTTP_Handler(srv NydusifyServiceHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in ListTasksRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationNydusifyServiceListTasks)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.ListTasks(ctx, req.(*ListTasksRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*ListTasksResponse)
		return ctx.Result(200, reply)
	}
}

func _NydusifyService_CancelTask0_HTTP_Handler(srv NydusifyServiceHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in CancelTaskRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		if err := ctx.BindVars(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationNydusifyServiceCancelTask)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.CancelTask(ctx, req.(*CancelTaskRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*CancelTaskResponse)
		return ctx.Result(200, reply)
	}
}

func _NydusifyService_RetryCallback0_HTTP_Handler(srv NydusifyServiceHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in RetryCallbackRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		if err := ctx.BindVars(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationNydusifyServiceRetryCallback)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.RetryCallback(ctx, req.(*RetryCallbackRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*RetryCallbackResponse)
		return ctx.Result(200, reply)
	}
}

type NydusifyServiceHTTPClient interface {
	CancelTask(ctx context.Context, req *CancelTaskRequest, opts ...http.CallOption) (rsp *CancelTaskResponse, err error)
	GetTaskStatus(ctx context.Context, req *GetTaskStatusRequest, opts ...http.CallOption) (rsp *GetTaskStatusResponse, err error)
	ListTasks(ctx context.Context, req *ListTasksRequest, opts ...http.CallOption) (rsp *ListTasksResponse, err error)
	RetryCallback(ctx context.Context, req *RetryCallbackRequest, opts ...http.CallOption) (rsp *RetryCallbackResponse, err error)
	SubmitCommitTask(ctx context.Context, req *SubmitCommitTaskRequest, opts ...http.CallOption) (rsp *SubmitCommitTaskResponse, err error)
}

type NydusifyServiceHTTPClientImpl struct {
	cc *http.Client
}

func NewNydusifyServiceHTTPClient(client *http.Client) NydusifyServiceHTTPClient {
	return &NydusifyServiceHTTPClientImpl{client}
}

func (c *NydusifyServiceHTTPClientImpl) CancelTask(ctx context.Context, in *CancelTaskRequest, opts ...http.CallOption) (*CancelTaskResponse, error) {
	var out CancelTaskResponse
	pattern := "/v1/nydusify/tasks/{task_id}"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationNydusifyServiceCancelTask))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "DELETE", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NydusifyServiceHTTPClientImpl) GetTaskStatus(ctx context.Context, in *GetTaskStatusRequest, opts ...http.CallOption) (*GetTaskStatusResponse, error) {
	var out GetTaskStatusResponse
	pattern := "/v1/nydusify/tasks/{task_id}"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationNydusifyServiceGetTaskStatus))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "GET", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NydusifyServiceHTTPClientImpl) ListTasks(ctx context.Context, in *ListTasksRequest, opts ...http.CallOption) (*ListTasksResponse, error) {
	var out ListTasksResponse
	pattern := "/v1/nydusify/tasks"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationNydusifyServiceListTasks))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "GET", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NydusifyServiceHTTPClientImpl) RetryCallback(ctx context.Context, in *RetryCallbackRequest, opts ...http.CallOption) (*RetryCallbackResponse, error) {
	var out RetryCallbackResponse
	pattern := "/v1/nydusify/tasks/{task_id}/retry-callback"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationNydusifyServiceRetryCallback))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NydusifyServiceHTTPClientImpl) SubmitCommitTask(ctx context.Context, in *SubmitCommitTaskRequest, opts ...http.CallOption) (*SubmitCommitTaskResponse, error) {
	var out SubmitCommitTaskResponse
	pattern := "/v1/nydusify/commit"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationNydusifyServiceSubmitCommitTask))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
