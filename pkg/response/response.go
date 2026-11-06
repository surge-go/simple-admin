package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/surge-go/simple-admin/pkg/consts"
	"github.com/surge-go/simple-admin/pkg/errors"
)

// Ok 返回成功响应。
func Ok(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, NewResponse(200, data, "success", traceID(ctx)))
	ctx.Abort()
	return
}

// Fail 返回失败响应。
func Fail(ctx *gin.Context, err error) {
	if err == nil {
		ctx.JSON(errors.ErrInternal.Status(), NewResponse[any](errors.ErrInternal.Code, nil, "unknown", traceID(ctx)))
		ctx.Abort()
		return
	}

	// 检查是否是 errors Error 类型
	var errorErr *errors.Error
	if errors.As(err, &errorErr) {
		// 使用 errors Error 的状态码和业务错误码
		ctx.JSON(errorErr.Status(), NewResponse[any](errorErr.Code, nil, errorErr.Message, traceID(ctx)))
		ctx.Abort()
		return
	}

	// 其他错误类型，默认返回 500
	ctx.JSON(errors.ErrInternal.Status(), NewResponse[any](errors.ErrInternal.Code, nil, errors.ErrInternal.Message, traceID(ctx)))
	ctx.Abort()
	return
}

// Nil 返回 nil 响应。
func Nil(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, NewResponse[any](0, nil, "nil", traceID(ctx)))
	ctx.Abort()
	return
}

// traceID 从上下文获取跟踪 ID。
func traceID(ctx *gin.Context) string {
	tid := ctx.GetString(consts.CtxTraceID)
	if tid == "" {
		return ""
	}
	return tid
}
