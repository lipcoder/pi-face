package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// 单条日志记录（严格使用当前 records.csv 格式：无表头、5 列）
// [0]=timestamp, [1]=match_name, [2]=similarity, [3]=threshold, [4]=status
type Record struct {
	ID         int    `json:"id"`
	Timestamp  string `json:"timestamp"`
	MatchName  string `json:"match_name"`
	Similarity string `json:"similarity"`
	Threshold  string `json:"threshold"`
	Status     string `json:"status"`
}

// 某人某日来了几次（按“同一人同一天只算一次”，Count 固定为 1）
type PersonDayCount struct {
	Person string `json:"person"`
	Date   string `json:"date"`
	Count  int    `json:"count"`
}

// 某日有几个人来（人数去重）
type DayPeopleCount struct {
	Date   string `json:"date"`
	People int    `json:"people"`
}

// 某月某人来的天数（天数去重）
type MonthPersonDays struct {
	Month  string `json:"month"`
	Person string `json:"person"`
	Days   int    `json:"days"`
}

// /api/stats 返回体（保持你前端 app.js 当前使用的字段：person_day、all_persons）
// 其中 person_day 是按人+日去重后的签到记录。
type StatsResponse struct {
	Total        int               `json:"total"`             // 原始总记录数
	MatchRaw     int               `json:"match_raw"`         // status=MATCH 的原始记录数（未去重）
	Valid        int               `json:"valid"`             // 按天去重后的有效签到次数（同一人同一天算一次）
	Error        int               `json:"error"`             // status=ERROR
	NoFace       int               `json:"no_face"`           // status=NO_FACE
	OtherInvalid int               `json:"other_invalid"`     // 其他非 MATCH 的状态
	PersonDay    []PersonDayCount  `json:"person_day"`        // 某人某日是否签到（一天只算一次）
	DayPeople    []DayPeopleCount  `json:"day_people"`        // 某日有几个人来（人数去重）
	MonthPerson  []MonthPersonDays `json:"month_person_days"` // 某月某人来的天数（天数去重）
	AllPersons   []string          `json:"all_persons"`       // 所有人员姓名（来自 label_map.json；若读取失败则为空）
	LabelMap     map[string]string `json:"label_map"`         // ID -> 姓名，对应 label_map.json
}

var (
	dataDir      string // data 目录根（相对路径），默认 ../data
	csvPath      string // 日志 CSV 文件路径（相对路径），默认 dataDir/logs/records.csv
	labelMapPath string // label_map.json 路径，默认 dataDir/feature_db/label_map.json
	staticDir    string // 前端静态资源目录，默认 ./static
)


// 格式：YYYY-MM-DD HH:MM:SS [INFO] message
var logFile *os.File

func initLogger(dataDir string) (string, error) {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}

	logPath := filepath.Join(logDir, "2.txt")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	logFile = f

	log.SetFlags(0)
	log.SetOutput(f)
	return logPath, nil
}

func closeLogger() {
	if logFile != nil {
		_ = logFile.Close()
	}
}

func logLine(level, msg string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	log.Printf("%s [%s] %s", ts, level, msg)
}

func logInfo(msg string)  { logLine("INFO", msg) }
func logError(msg string) { logLine("ERROR", msg) }

func logInfof(format string, args ...any) {
	logLine("INFO", fmt.Sprintf(format, args...))
}

func logErrorf(format string, args ...any) {
	logLine("ERROR", fmt.Sprintf(format, args...))
}

func logSep() {
	logInfo("===========================================================")
}


// API 访问日志（含来源 IP、状态码、耗时）
func clientIP(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 200}

		ip := clientIP(r)

		next.ServeHTTP(sr, r)

		cost := time.Since(start).Milliseconds()
		logInfof("API ip=%s %s %s status=%d cost=%dms ua=%q",
			ip, r.Method, r.URL.Path, sr.status, cost, r.UserAgent())
	})
}

func main() {
	dataDir = os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "../data"
	}

	logPath, err := initLogger(dataDir)
	if err != nil {
		log.SetFlags(0)
		log.Printf("logger init failed: %v", err)
	} else {
		logInfof("log file: %s", logPath)
	}
	defer closeLogger()

	csvPath = os.Getenv("RECORDS_CSV_PATH")
	if csvPath == "" {
		csvPath = filepath.Join(dataDir, "logs", "records.csv")
	}

	labelMapPath = os.Getenv("LABEL_MAP_PATH")
	if labelMapPath == "" {
		labelMapPath = filepath.Join(dataDir, "feature_db", "label_map.json")
	}

	staticDir = os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./static"
	}

	logSep()
	logInfof("使用 data 目录: %s", dataDir)
	logInfof("使用 CSV 日志: %s", csvPath)
	logInfof("使用 label_map: %s", labelMapPath)
	logInfof("使用静态目录: %s", staticDir)
	logSep()

	mux := http.NewServeMux()

	// 前端静态文件
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", fs)

	// API
	mux.HandleFunc("/api/records", handleRecords)
	mux.HandleFunc("/api/stats", handleStats)

	// 图片预览：/image?path=unknow/xxx.jpg （相对于 dataDir）
	mux.HandleFunc("/image", handleImage)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: loggingMiddleware(mux),
	}

	// 捕获 SIGINT/SIGTERM，记录关闭日志
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logSep()
		logInfo("服务启动成功：http://0.0.0.0:8080")
		logSep()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logErrorf("server listen error: %v", err)
		}
	}()

	sig := <-stop
	logSep()
	logInfof("收到关闭信号: %s", sig.String())
	logInfo("开始优雅关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logErrorf("优雅关闭失败: %v", err)
	} else {
		logInfo("服务已关闭")
	}
	logSep()
}

// 每次请求重新读取 CSV，保证看到最新记录
// 严格要求：无表头、且每行必须恰好 5 列（多/少都跳过）
// [0]=timestamp, [1]=match_name, [2]=similarity, [3]=threshold, [4]=status
func loadRecordsFromCSV(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	var result []Record
	id := 1

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// 严格：只接受 5 列
		if len(row) != 5 {
			continue
		}

		rec := Record{
			ID:         id,
			Timestamp:  strings.TrimSpace(row[0]),
			MatchName:  strings.TrimSpace(row[1]),
			Similarity: strings.TrimSpace(row[2]),
			Threshold:  strings.TrimSpace(row[3]),
			Status:     strings.TrimSpace(row[4]),
		}
		result = append(result, rec)
		id++
	}
	return result, nil
}

// 读取 label_map.json，返回 ID -> 姓名 的映射
func loadLabelMap(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var m map[string]string
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// /api/records?status=MATCH|ERROR|NO_FACE&q=...&page=1&pageSize=20
// 列表只是原始行，不做按天去重，方便排查
func handleRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	records, err := loadRecordsFromCSV(csvPath)
	if err != nil {
		logErrorf("读取 CSV 失败: %v", err)
		resp := struct {
			Data     []Record `json:"data"`
			Total    int      `json:"total"`
			Page     int      `json:"page"`
			PageSize int      `json:"pageSize"`
		}{
			Data:     []Record{},
			Total:    0,
			Page:     1,
			PageSize: 20,
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// 解析分页参数
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("pageSize"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	// 过滤条件
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	var filtered []Record
	for _, rec := range records {
		s := strings.TrimSpace(rec.Status)

		// 状态过滤
		if statusFilter != "" && !strings.EqualFold(s, statusFilter) {
			continue
		}

		// 模糊搜索（按姓名/状态）
		if search != "" {
			if !containsFold(rec.MatchName, search) && !containsFold(rec.Status, search) {
				continue
			}
		}

		filtered = append(filtered, rec)
	}

	// 按时间倒序排序
	sort.Slice(filtered, func(i, j int) bool {
		ti, err1 := parseTimestamp(filtered[i].Timestamp)
		tj, err2 := parseTimestamp(filtered[j].Timestamp)
		if err1 != nil && err2 != nil {
			return filtered[i].ID > filtered[j].ID
		}
		if err1 != nil {
			return false
		}
		if err2 != nil {
			return true
		}
		return ti.After(tj)
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	resp := struct {
		Data     []Record `json:"data"`
		Total    int      `json:"total"`
		Page     int      `json:"page"`
		PageSize int      `json:"pageSize"`
	}{
		Data:     filtered[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// /api/stats 统计接口：按“同一人同一天只算一次”
func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	records, err := loadRecordsFromCSV(csvPath)
	if err != nil {
		logErrorf("读取 CSV 失败: %v", err)
		_ = json.NewEncoder(w).Encode(StatsResponse{})
		return
	}

	var stats StatsResponse
	stats.Total = len(records)

	// 统计原始状态数量 + 按“同一人同一天”去重的签到
	personDaySet := make(map[string]map[string]struct{})                   // person -> set(date)
	dayPeopleSet := make(map[string]map[string]struct{})                  // date -> set(person)
	monthPersonDaysSet := make(map[string]map[string]map[string]struct{}) // month -> person -> set(date)

	for _, rec := range records {
		sTrim := strings.TrimSpace(rec.Status)
		upperStatus := strings.ToUpper(sTrim)

		switch upperStatus {
		case "MATCH":
			stats.MatchRaw++

			t, err := parseTimestamp(rec.Timestamp)
			if err != nil {
				continue
			}

			// 过滤掉 UNKNOWN / NO_FACE / 空名字，只统计真实人员
			rawName := strings.TrimSpace(rec.MatchName)
			upperName := strings.ToUpper(rawName)
			if rawName == "" || upperName == "UNKNOWN" || upperName == "NO_FACE" {
				continue
			}
			person := rawName

			date := t.Format("2006-01-02")
			month := t.Format("2006-01")

			if _, ok := personDaySet[person]; !ok {
				personDaySet[person] = make(map[string]struct{})
			}
			if _, exists := personDaySet[person][date]; exists {
				continue
			}

			personDaySet[person][date] = struct{}{}
			stats.Valid++

			if _, ok := dayPeopleSet[date]; !ok {
				dayPeopleSet[date] = make(map[string]struct{})
			}
			dayPeopleSet[date][person] = struct{}{}

			if _, ok := monthPersonDaysSet[month]; !ok {
				monthPersonDaysSet[month] = make(map[string]map[string]struct{})
			}
			if _, ok := monthPersonDaysSet[month][person]; !ok {
				monthPersonDaysSet[month][person] = make(map[string]struct{})
			}
			monthPersonDaysSet[month][person][date] = struct{}{}

		case "ERROR":
			stats.Error++
		case "NO_FACE":
			stats.NoFace++
		default:
			if upperStatus != "" {
				stats.OtherInvalid++
			}
		}
	}

	for person, daySet := range personDaySet {
		for date := range daySet {
			stats.PersonDay = append(stats.PersonDay, PersonDayCount{
				Person: person,
				Date:   date,
				Count:  1,
			})
		}
	}

	for date, set := range dayPeopleSet {
		stats.DayPeople = append(stats.DayPeople, DayPeopleCount{
			Date:   date,
			People: len(set),
		})
	}

	for month, personSet := range monthPersonDaysSet {
		for person, daySet := range personSet {
			stats.MonthPerson = append(stats.MonthPerson, MonthPersonDays{
				Month:  month,
				Person: person,
				Days:   len(daySet),
			})
		}
	}

	sort.Slice(stats.PersonDay, func(i, j int) bool {
		if stats.PersonDay[i].Person == stats.PersonDay[j].Person {
			return stats.PersonDay[i].Date < stats.PersonDay[j].Date
		}
		return stats.PersonDay[i].Person < stats.PersonDay[j].Person
	})

	sort.Slice(stats.DayPeople, func(i, j int) bool {
		return stats.DayPeople[i].Date < stats.DayPeople[j].Date
	})

	sort.Slice(stats.MonthPerson, func(i, j int) bool {
		if stats.MonthPerson[i].Month == stats.MonthPerson[j].Month {
			return stats.MonthPerson[i].Person < stats.MonthPerson[j].Person
		}
		return stats.MonthPerson[i].Month < stats.MonthPerson[j].Month
	})

	// 读取 label_map.json（可选）：如果失败，不阻断接口
	if labelMap, err := loadLabelMap(labelMapPath); err == nil {
		stats.LabelMap = labelMap
		nameSet := make(map[string]struct{})
		for _, name := range labelMap {
			n := strings.TrimSpace(name)
			if n == "" {
				continue
			}
			nameSet[n] = struct{}{}
		}
		for name := range nameSet {
			stats.AllPersons = append(stats.AllPersons, name)
		}
		sort.Strings(stats.AllPersons)
	} else {
		// 不报错给前端，但记录到 2.txt
		logErrorf("读取 label_map 失败: %v", err)
		stats.AllPersons = []string{}
		stats.LabelMap = map[string]string{}
	}

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// 图片预览：/image?path=unknow/xxx.jpg （相对于 dataDir）
func handleImage(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(dataDir, path)
	f, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(fullPath)) {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	_, _ = io.Copy(w, f)
}

// 工具函数

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func parseTimestamp(ts string) (time.Time, error) {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Time{}, errors.New("empty timestamp")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
		"2006-01-02",
		"2006/01/02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}

	// 如果有小数秒，截掉小数部分再试
	if i := strings.Index(ts, "."); i != -1 {
		ts2 := ts[:i]
		for _, layout := range layouts {
			if t, err := time.Parse(layout, ts2); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, errors.New("cannot parse timestamp")
}
