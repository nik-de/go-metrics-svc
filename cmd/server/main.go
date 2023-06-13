package main

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// MetricType определяет тип метрики.
type MetricType int

const (
	GaugeType MetricType = iota
	CounterType
)

// Metric определяет структуру метрики.
type Metric struct {
	Name  string
	Type  MetricType
	Value interface{}
}

// MemStorage определяет интерфейс хранилища метрик.
type MemStorage interface {
	Add(m Metric)
	Get() []Metric
}

// MemStorageImpl - тип для хранения метрик.
type MemStorageImpl struct {
	sync.RWMutex
	metrics []Metric
}

// NewMemStorage создает новое хранилище метрик.
func NewMemStorage() *MemStorageImpl {
	return &MemStorageImpl{}
}

// Add добавляет новую метрику или обновляет значение существующей метрики.
func (s *MemStorageImpl) Add(m Metric) {
	s.Lock()
	defer s.Unlock()

	for i := range s.metrics {
		if s.metrics[i].Name == m.Name && s.metrics[i].Type == m.Type {
			switch m.Type {
			case GaugeType:
				s.metrics[i].Value = m.Value
			case CounterType:
				s.metrics[i].Value = s.metrics[i].Value.(int64) + m.Value.(int64)
			}
			return
		}
	}

	s.metrics = append(s.metrics, m)
}

// Get возвращает список всех метрик.
func (s *MemStorageImpl) Get() []Metric {
	s.RLock()
	defer s.RUnlock()

	metrics := make([]Metric, len(s.metrics))
	copy(metrics, s.metrics)
	return metrics
}

func main() {
	// создаем новое хранилище метрик
	storage := NewMemStorage()

	// создаем роутер gin
	router := gin.Default()
	// обработчик ошибок
	router.Use(func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				log.Println("Recovered panic:", r)
			}
		}()
		c.Next()
	})

	// обработчик запросов на обновление метрик
	router.POST("/update/:type/:name/:value", func(c *gin.Context) {
		metricType := c.Param("type")
		metricName := c.Param("name")
		metricValueStr := c.Param("value")

		var metricValue interface{}
		var err error

		if metricType == "gauge" {
			metricValue, err = strconv.ParseFloat(metricValueStr, 64)
		} else if metricType == "counter" {
			metricValue, err = strconv.ParseInt(metricValueStr, 10, 64)
		} else {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid metric type"})
			return
		}

		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid metric value"})
			return
		}

		metric := Metric{
			Name:  metricName,
			Type:  GaugeType,
			Value: metricValue,
		}
		if metricType == "counter" {
			metric.Type = CounterType
		}

		storage.Add(metric)

		c.Status(http.StatusOK)
	})

	// обработчик запросов на получение метрик
	router.GET("/metrics", func(c *gin.Context) {
		metrics := storage.Get()

		for _, metric := range metrics {
			c.String(http.StatusOK, "%s: %v\n", metric.Name, metric.Value)
		}
	})

	// запускаем сервер на порту 8080
	log.Fatal(http.ListenAndServe(":8080", router))
}
