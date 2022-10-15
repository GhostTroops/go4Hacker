package ginutils

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
)

var ErrNotExist = errors.New("Not Exist")

func GetQueryInt(c *gin.Context, q string) (int, error) {
	var v int
	queryValue, exist := c.GetQuery(q)
	if !exist {
		return 0, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%d", &v)
	return v, err
}

func GetQueryInt64(c *gin.Context, q string) (int64, error) {
	var v int64
	queryValue, exist := c.GetQuery(q)
	if !exist {
		return 0, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%d", &v)
	return v, err
}

func GetQueryfloat(c *gin.Context, q string) (float32, error) {
	var v float32

	queryValue, exist := c.GetQuery(q)
	if !exist {
		return v, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%f", &v)
	return v, err
}

func GetQueryfloat64(c *gin.Context, q string) (float64, error) {
	var v float64
	queryValue, exist := c.GetQuery(q)
	if !exist {
		return v, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%f", &v)
	return v, err
}

func GetQueryBoolean(c *gin.Context, q string) (bool, error) {
	queryValue, exist := c.GetQuery(q)
	if !exist {
		return false, ErrNotExist
	}
	switch strings.ToLower(queryValue) {
	case "true", "yes", "y":
		return true, nil
	case "false", "no", "n":
		return false, nil
	}

	return false, errors.Errorf("unexpect value=%v", queryValue)
}

func GetHeaderInt(c *gin.Context, q string) (int, error) {
	var v int
	queryValue := c.GetHeader(q)
	if queryValue == "" {
		return v, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%d", &v)
	return v, err
}

func GetHeaderInt64(c *gin.Context, q string) (int64, error) {
	var v int64
	queryValue := c.GetHeader(q)
	if queryValue == "" {
		return v, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%d", &v)
	return v, err
}

func GetHeaderfloat(c *gin.Context, q string) (float32, error) {
	var v float32
	queryValue := c.GetHeader(q)
	if queryValue == "" {
		return 0, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%f", &v)
	return v, err
}

func GetHeaderfloat64(c *gin.Context, q string) (float64, error) {
	var v float64
	queryValue := c.GetHeader(q)
	if queryValue == "" {
		return v, ErrNotExist
	}
	_, err := fmt.Sscanf(queryValue, "%f", &v)
	return v, err
}

func GetHeaderBoolean(c *gin.Context, q string) (bool, error) {
	queryValue := c.GetHeader(q)
	if queryValue == "" {
		return false, ErrNotExist
	}
	switch strings.ToLower(queryValue) {
	case "true", "yes", "y":
		return true, nil
	case "false", "no", "n":
		return false, nil
	}

	return false, errors.Errorf("unexpect value=%v", queryValue)
}
