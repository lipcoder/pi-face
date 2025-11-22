package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Record struct {
	ID         int    `json:"id"`
	Timestamp  string `json:"timestamp"`
	ImagePath  string `json:"image_path"`
	MatchName  string `json:"match_name"`
	Similarity string `json:"similarity"`
	Threshold  string `json:"threshold"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

// 某人某日来了几次（按 1 小时去重之后的“有效签到次数”）
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

type StatsResponse struct {
	Total        int               `json:"total"`           // 原始总记录数
	MatchRaw     int               `json:"match_raw"`       // status=MATCH 的原始记录数（未去重）
	Valid        int               `json:"valid"`           // 按 1 小时去重后的有效签到次数
	Error        int               `json:"error"`           // status=ERROR
	NoFace       int               `json:"no_face"`         // status=NO_FACE
	OtherInvalid int               `json:"other_invalid"`   // 其他非 MATCH 的状态
	PersonDay    []PersonDayCount  `json:"person_day"`      // 某人某日来了几次
	DayPeople    []DayPeopleCount  `json:"day_people"`      // 某日有几个人来
	MonthPerson  []MonthPersonDays `json:"month_person_days"` // 某月某人来的天数
}

var csvPath string
func main() {
	// 日志 CSV 绝对路径，默认 /data/logs/records.csv，可用环境变量覆盖
	csvPath = os.Getenv("RECORDS_CSV_PATH")
	if csvPath == "" {
		csvPath = "/data/logs/records.csv"
	}

	log.Printf("使用日志文件: %s", csvPath)

	mux := http.NewServeMux()

	// 前端静态文件（/app/web/static）
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	// API
	mux.HandleFunc("/api/records", handleRecords)
	mux.HandleFunc("/api/stats", handleStats)

	// 图片预览：/image?path=/data/unknow/xxx.jpg
	mux.HandleFunc("/image", handleImage)

	log.Println("服务启动成功：http://0.0.0.0:8080")
	if err := http.ListenAndServe(":8080", loggingMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

// 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// 每次请求重新从绝对路径读取 CSV，保证看到最新记录
func loadRecordsFromCSV(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	header, err := reader.Read()
	if err == io.EOF {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}

	index := map[string]int{}
	for i, h := range header {
		h = strings.TrimSpace(h)
		index[h] = i
	}

	get := func(row []string, key string) string {
		i, ok := index[key]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

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

		rec := Record{
			ID:         id,
			Timestamp:  get(row, "timestamp"),
			ImagePath:  get(row, "image_path"),
			MatchName:  get(row, "match_name"),
			Similarity: get(row, "similarity"),
			Threshold:  get(row, "threshold"),
			Status:     get(row, "status"),
			Message:    get(row, "message"),
		}
		result = append(result, rec)
		id++
	}

	return result, nil
}
// /api/records?status=MATCH|ERROR|NO_FACE&q=...&page=1&pageSize=20
// 列表只是原始行，不做 1 小时去重，方便排查
func handleRecords(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	records, err := loadRecordsFromCSV(csvPath)
	if err != nil {
		log.Printf("读取 CSV 失败: %v", err)
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

	q := r.URL.Query()
	statusFilter := strings.TrimSpace(q.Get("status"))
	search := strings.ToLower(strings.TrimSpace(q.Get("q")))

	page := parseInt(q.Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseInt(q.Get("pageSize"), 20)
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 20
	}

	var filtered []Record
	for _, rec := range records {
		s := strings.TrimSpace(rec.Status)

		// 状态过滤
		if statusFilter != "" && !strings.EqualFold(s, statusFilter) {
			continue
		}

		// 模糊搜索
		if search != "" {
			if !containsFold(rec.MatchName, search) &&
				!containsFold(rec.ImagePath, search) &&
				!containsFold(rec.Message, search) &&
				!containsFold(rec.Status, search) {
				continue
			}
		}

		filtered = append(filtered, rec)
	}

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
		return
	}
}
// /api/stats
// 图表只基于 status=MATCH 的记录，并且对“同一个人 1 小时内的签到”合并为一次有效签到
func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	records, err := loadRecordsFromCSV(csvPath)
	if err != nil {
		log.Printf("读取 CSV 失败: %v", err)
		_ = json.NewEncoder(w).Encode(StatsResponse{})
		return
	}

	var stats StatsResponse
	stats.Total = len(records)

	type timedRec struct {
		rec Record
		t   time.Time
	}

	matchRaw := 0
	personMap := make(map[string][]timedRec)

	// 先按 status 分类，并把 status=MATCH 的记录按人聚合，解析时间
	for _, rec := range records {
		sTrim := strings.TrimSpace(rec.Status)
		upper := strings.ToUpper(sTrim)

		switch upper {
		case "MATCH":
			matchRaw++
			t, err := parseTimestamp(rec.Timestamp)
			if err != nil {
				// 时间解析失败的 MATCH 记录，只记在 matchRaw，不参与时间统计
				continue
			}
			person := strings.TrimSpace(rec.MatchName)
			if person == "" {
				person = "未知"
			}
			personMap[person] = append(personMap[person], timedRec{rec: rec, t: t})
		case "ERROR":
			stats.Error++
		case "NO_FACE":
			stats.NoFace++
		default:
			if sTrim != "" {
				stats.OtherInvalid++
			}
		}
	}

	stats.MatchRaw = matchRaw

	// 用于统计：
	personDayMap := make(map[string]int)                                // person|date -> count
	dayPeopleMap := make(map[string]map[string]struct{})                // date -> set(person)
	monthPersonDaysMap := make(map[string]map[string]map[string]struct{}) // month -> person -> set(date)

	// 对每个人做时间排序 & 1 小时去重
	for person, list := range personMap {
		sort.Slice(list, func(i, j int) bool {
			return list[i].t.Before(list[j].t)
		})

		var lastVisit time.Time
		hasLast := false

		for _, item := range list {
			if !hasLast || item.t.Sub(lastVisit) > time.Hour {
				// 这是一次“有效签到”
				stats.Valid++
				hasLast = true
				lastVisit = item.t

				date := item.t.Format("2006-01-02")
				month := item.t.Format("2006-01")

				// 某人某日来了几次
				key := person + "|" + date
				personDayMap[key]++

				// 某日有几个人来（去重）
				if _, ok := dayPeopleMap[date]; !ok {
					dayPeopleMap[date] = make(map[string]struct{})
				}
				dayPeopleMap[date][person] = struct{}{}

				// 某月某人来的天数（去重天数）
				if _, ok := monthPersonDaysMap[month]; !ok {
					monthPersonDaysMap[month] = make(map[string]map[string]struct{})
				}
				if _, ok := monthPersonDaysMap[month][person]; !ok {
					monthPersonDaysMap[month][person] = make(map[string]struct{})
				}
				monthPersonDaysMap[month][person][date] = struct{}{}
			}
			// 1 小时内的重复签到直接跳过，不更新 lastVisit（窗口以上一次有效签到为基准）
		}
	}

	// 展开 personDayMap
	for key, count := range personDayMap {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		stats.PersonDay = append(stats.PersonDay, PersonDayCount{
			Person: parts[0],
			Date:   parts[1],
			Count:  count,
		})
	}

	// 展开 dayPeopleMap
	for date, set := range dayPeopleMap {
		stats.DayPeople = append(stats.DayPeople, DayPeopleCount{
			Date:   date,
			People: len(set),
		})
	}

	// 展开 monthPersonDaysMap
	for month, personSet := range monthPersonDaysMap {
		for person, daySet := range personSet {
			stats.MonthPerson = append(stats.MonthPerson, MonthPersonDays{
				Month:  month,
				Person: person,
				Days:   len(daySet),
			})
		}
	}

	// 排序，方便前端展示
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

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
// 图片接口：/image?path=/data/unknow/xxx.jpg
// 限制只能访问 /data/ 下的文件
func handleImage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(path, "/data/") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, path)
}

// 工具函数
func parseInt(s string, def int) int {
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
	return strings.Contains(strings.ToLower(s), substr)
}

// 尝试解析各种常见时间格式
func parseTimestamp(ts string) (time.Time, error) {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Time{}, errors.New("empty timestamp")
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04",
	}

	// 先尝试完整字符串
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
