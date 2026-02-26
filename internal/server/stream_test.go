package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockTokenProducer 是一个模拟的TokenProducer
type mockTokenProducer struct {
	mock.Mock
}

func (m *mockTokenProducer) Produce(yield func(string) bool) error {
	args := m.Called(yield)
	return args.Error(0)
}

// mockFlushWriter 实现 http.ResponseWriter 和 http.Flusher 接口
type mockFlushWriter struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (m *mockFlushWriter) Flush() {
	m.flushCount++
}

func TestWritePlainTokens_Success(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// 定义测试数据
	tokens := []string{"Hello", " ", "World", "!"}
	expectedResponse := strings.Join(tokens, "")

	// 调用WritePlainTokens函数
	response, err := WritePlainTokens(c, func(yield func(string) bool) error {
		for _, token := range tokens {
			if !yield(token) {
				break
			}
		}
		return nil
	}, StreamOptions{})

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	assert.Equal(t, expectedResponse, w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}

func TestWritePlainTokens_ErrorAtBeginning(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// 定义测试错误
	testErr := errors.New("test error")

	// 调用WritePlainTokens函数
	response, err := WritePlainTokens(c, func(yield func(string) bool) error {
		return testErr
	}, StreamOptions{})

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, "", response)
	assert.Equal(t, "ERROR: test error", w.Body.String())
}

func TestWritePlainTokens_ErrorAfterOutput(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// 定义测试数据
	initialTokens := []string{"Hello", " "}
	expectedResponse := strings.Join(initialTokens, "")
	testErr := errors.New("test error")

	// 调用WritePlainTokens函数
	response, err := WritePlainTokens(c, func(yield func(string) bool) error {
		for _, token := range initialTokens {
			if !yield(token) {
				break
			}
		}
		return testErr
	}, StreamOptions{})

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, expectedResponse, response)
	assert.Equal(t, expectedResponse, w.Body.String()) // 错误不应该覆盖已有输出
}

func TestWritePlainTokens_CancelledContext(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建带有取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(ctx)

	// 立即取消上下文
	cancel()

	// 调用WritePlainTokens函数
	response, err := WritePlainTokens(c, func(yield func(string) bool) error {
		for i := 0; i < 10; i++ {
			if !yield("token") {
				break
			}
		}
		return nil
	}, StreamOptions{})

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, "", response) // 应该没有输出，因为第一次调用就会被取消
}

func TestWritePlainTokens_EmptyTokensFiltered(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// 定义包含空token的测试数据
	tokens := []string{"Hello", "", " ", "", "World", ""}
	expectedResponse := "Hello World" // 空token应该被过滤掉，保留空格

	// 调用WritePlainTokens函数
	response, err := WritePlainTokens(c, func(yield func(string) bool) error {
		for _, token := range tokens {
			if !yield(token) {
				break
			}
		}
		return nil
	}, StreamOptions{})

	// 验证结果
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	assert.Equal(t, expectedResponse, w.Body.String())
}

func TestWritePlainTokens_CustomOptions(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建响应记录器和Gin上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// 定义自定义选项
	customOpts := StreamOptions{
		ContentType:  "text/event-stream",
		CacheControl: "no-store",
		ErrorPrefix:  "STREAM_ERROR: ",
	}

	// 定义测试错误
	testErr := errors.New("custom error")

	// 调用WritePlainTokens函数
	_, err := WritePlainTokens(c, func(yield func(string) bool) error {
		return testErr
	}, customOpts)

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Equal(t, "STREAM_ERROR: custom error", w.Body.String())
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}