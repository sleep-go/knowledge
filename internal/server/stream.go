package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// StreamOptions 配置流式响应的行为
type StreamOptions struct {
	// ContentType 设置响应的内容类型，默认为"text/plain; charset=utf-8"
	ContentType string
	
	// CacheControl 设置缓存控制头，默认为"no-cache"
	CacheControl string
	
	// ErrorPrefix 设置错误信息的前缀，默认为"ERROR: "
	ErrorPrefix string
}

// TokenProducer 定义生成token的函数签名
type TokenProducer func(yield func(string) bool) error

// WritePlainTokens 统一流式响应处理函数
// 该函数处理流式输出的所有通用逻辑，包括：
// - 检查HTTP Flushers支持
// - 设置响应头
// - 处理每个token的输出和flush
// - 处理取消和错误情况
// - 返回完整响应内容用于存储
func WritePlainTokens(c *gin.Context, produce TokenProducer, opts StreamOptions) (string, error) {
	// 设置默认值
	if opts.ContentType == "" {
		opts.ContentType = "text/plain; charset=utf-8"
	}
	if opts.CacheControl == "" {
		opts.CacheControl = "no-cache"
	}
	if opts.ErrorPrefix == "" {
		opts.ErrorPrefix = "ERROR: "
	}

	// 检查是否支持流式输出
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return "", fmt.Errorf("streaming not supported")
	}

	// 设置响应头
	c.Header("Content-Type", opts.ContentType)
	c.Header("Cache-Control", opts.CacheControl)
	c.Status(http.StatusOK)

	// 创建字符串构建器来收集完整响应
	var out strings.Builder
	
	// 执行token生产函数
	err := produce(func(token string) bool {
		// 检查请求是否已取消
		select {
		case <-c.Request.Context().Done():
			return false
		default:
		}
		
		// 过滤空token
		if token == "" {
			return true
		}
		
		// 累积token到完整响应中
		out.WriteString(token)
		
		// 写入token到响应并flush
		_, _ = c.Writer.WriteString(token)
		flusher.Flush()
		
		return true
	})

	// 处理错误情况
	if err != nil {
		// 如果没有输出任何内容，则显示错误信息
		if out.Len() == 0 {
			_, _ = c.Writer.WriteString(opts.ErrorPrefix + err.Error())
			flusher.Flush()
		}
		return out.String(), err
	}

	// 如果没有输出任何内容且没有错误，这也是一种异常情况
	if out.Len() == 0 {
		select {
		case <-c.Request.Context().Done():
			return "", nil
		default:
		}
		errMsg := "Model produced no output (check logs for details)"
		_, _ = c.Writer.WriteString(opts.ErrorPrefix + errMsg)
		flusher.Flush()
		return "", fmt.Errorf("%s", errMsg)
	}

	return out.String(), nil
}
