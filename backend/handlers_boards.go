package backend

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxAnnouncementBody = 8000
const maxCalendarNote = 2000

// ---------------------------------------------------------------------------
// Seeds
// ---------------------------------------------------------------------------

// seedQuotes feeds the dashboard "thought of the day" bubble. Entries are
// one-line psychology takeaways with their classic sources, bilingual so the
// frontend picks by the user's language.
var seedQuotes = [][3]string{
	{"神奇数字 7±2：短时记忆一次只能抓住几块信息。", "The magical number seven, plus or minus two: short-term memory holds only a few chunks at a time.", "Miller, 1956"},
	{"遗忘曲线在刚学完时最陡——及时复习最划算。", "The forgetting curve is steepest right after learning: reviewing early pays the most.", "Ebbinghaus, 1885"},
	{"一起放电的神经元，会连在一起。", "Neurons that fire together, wire together.", "Hebb, 1949"},
	{"记忆不是回放，而是重构——每次回忆都是一次重写。", "Memory is reconstruction, not playback: every recall rewrites the trace.", "Bartlett, 1932"},
	{"睡眠不是浪费时间，那是大脑在巩固一天的记忆。", "Sleep is when the brain consolidates the day's memories.", "记忆巩固研究"},
	{"Stroop 效应：阅读自动化到你想抑制都抑制不住。", "The Stroop effect: reading is so automatic you cannot suppress it.", "Stroop, 1935"},
	{"旁观者效应：在场的人越多，每个人出手相助的可能反而越小。", "The more bystanders present, the less likely any one of them helps.", "Darley & Latané, 1968"},
	{"我们会对眼前的巨变视而不见——这叫无意视盲。", "We are blind to salient changes right in front of us: inattentional blindness.", "Simons & Chabris, 1999"},
	{"尝试回忆比反复阅读更巩固记忆——这是测试效应。", "Retrieval practice beats rereading: the testing effect.", "Roediger & Karpicke, 2006"},
	{"间隔学习比集中突击更有效，哪怕总时间完全一样。", "Spaced practice beats massed cramming, even with equal total time.", "Cepeda et al., 2006"},
	{"多巴胺不只是快乐分子，更是“期待”的分子。", "Dopamine is less about pleasure and more about anticipation.", "Schultz, 1997"},
	{"镜像神经元让我们不用说话也能彼此理解。", "Mirror neurons let us understand each other without words.", "Rizzolatti et al., 1996"},
	{"情境依赖记忆：在学的地方考，成绩往往更好。", "Context-dependent memory: testing in the room where you learned helps.", "Godden & Baddeley, 1975"},
	{"情绪唤起的事件记得更牢——杏仁核在给记忆“盖章”。", "Emotionally arousing events stick: the amygdala stamps memories in.", "McGaugh, 2004"},
	{"前额叶皮质要到 25 岁左右才发育成熟——冲动有生理原因。", "The prefrontal cortex keeps maturing into the mid-twenties.", "神经发育研究"},
	{"确认偏误：我们擅长找支持自己的证据，忽略相反的。", "Confirmation bias: we hunt for evidence that flatters our beliefs.", "Wason, 1960"},
	{"可得性启发：越容易想起的事，我们越觉得它常见。", "Availability heuristic: the easier it comes to mind, the more common it feels.", "Tversky & Kahneman, 1973"},
	{"锚定效应：最先看到的数字会悄悄拖动你的判断。", "Anchoring: the first number you see drags your judgment.", "Tversky & Kahneman, 1974"},
	{"巴甫洛夫的狗告诉我们：联结学习无处不在。", "Pavlov's dogs show associative learning is everywhere.", "Pavlov, 1927"},
	{"斯金纳箱：行为的后果塑造行为本身。", "Skinner's box: consequences shape behavior.", "Skinner, 1938"},
	{"观察学习：看见别人被奖赏，我们也更可能去模仿。", "Observational learning: we imitate what we see rewarded.", "Bandura, 1961"},
	{"习得性无助：反复的失控感会让人放弃尝试。", "Learned helplessness: repeated loss of control teaches giving up.", "Seligman & Maier, 1967"},
	{"认知失调：行为改变态度，比态度改变行为更常见。", "Cognitive dissonance: behavior often rewrites attitude.", "Festinger, 1957"},
	{"从众实验：面对群体压力，约三分之一的人会否认显而易见的答案。", "Under group pressure, about a third deny the obvious.", "Asch, 1951"},
	{"服从实验：普通人在权威指令下可能做出违背良心的事。", "Ordinary people can follow authority against conscience.", "Milgram, 1963"},
	{"基本归因错误：解释别人看人品，解释自己看处境。", "Fundamental attribution error: others' acts are character, ours are circumstances.", "Ross, 1977"},
	{"自利偏差：赢了我厉害，输了运气差。", "Self-serving bias: wins are skill, losses are luck.", "社会心理学"},
	{"皮格马利翁效应：老师期待高的学生，真的进步更快。", "The Pygmalion effect: expectations shape outcomes.", "Rosenthal & Jacobson, 1968"},
	{"系列位置效应：开头和结尾的内容最好记。", "Serial position effect: beginnings and endings are remembered best.", "Murdock, 1962"},
	{"组块化是记忆的杠杆：拆成有意义的块，容量立刻翻倍。", "Chunking multiplies memory capacity.", "Miller, 1956"},
	{"心流：当难度刚好高出能力一点点，时间会消失。", "Flow arrives when challenge slightly exceeds skill.", "Csikszentmihalyi, 1990"},
	{"耶克斯–多德森定律：中等唤起水平下表现最好。", "Yerkes–Dodson law: moderate arousal wins.", "Yerkes & Dodson, 1908"},
	{"皮质醇短期提高警觉，长期却伤害海马。", "Cortisol sharpens the short run but erodes the hippocampus long term.", "Sapolsky, 2004"},
	{"安慰剂效应是真实的生理现象，不只是心理作用。", "The placebo effect is real physiology, not imagination.", "Benedetti, 2009"},
	{"视网膜只有中央凹是高清的——眼睛每秒都在跳动补全世界。", "Only the fovea sees in HD; the eyes saccade to fill the world in.", "视觉注意研究"},
	{"错觉相关：我们会在随机里看出规律。", "Illusory correlation: we see patterns in randomness.", "Chapman & Chapman, 1967"},
	{"自我实现预言：被相信会发生的事，更可能真的发生。", "Self-fulfilling prophecy: belief nudges reality.", "Merton, 1948"},
	{"群体极化：讨论让群体的观点比任何成员都更极端。", "Group discussion pushes views further than any member started.", "Moscovici & Zavalloni, 1969"},
	{"社会惰化：人越多，每个人反而越少使劲。", "Social loafing: more hands, less effort each.", "Latané et al., 1979"},
	{"手写笔记比打字记得牢：提炼比照录更费脑，也更深。", "Handwritten notes beat typed ones: summarizing is deeper processing.", "Mueller & Oppenheimer, 2014"},
}

// seedBoards installs the default dashboard board content on empty tables
// only: one welcome announcement (so the board is never blank on a fresh
// install) and the quote pack behind the daily thought card.
func seedBoards(db *gorm.DB) error {
	var announcements int64
	db.Model(&Announcement{}).Count(&announcements)
	if announcements == 0 {
		welcome := Announcement{
			ID:         NewID("ann"),
			Title:      "欢迎来到 Psych Hub",
			Body:       "这里是我们的心理学小基地。\n\n先去【词条】里点亮你的第一批概念吧！老师发布的 ==tangerine|公告和截止日期== 也会出现在这个看板上。",
			Pinned:     true,
			Status:     "published",
			AuthorName: "Psych Hub",
		}
		if err := db.Create(&welcome).Error; err != nil {
			return err
		}
	}
	var quotes int64
	db.Model(&Quote{}).Count(&quotes)
	if quotes == 0 {
		for _, q := range seedQuotes {
			row := Quote{TextZh: q[0], TextEn: q[1], Source: q[2]}
			if err := db.Create(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// quoteOfToday picks the deterministic quote of the day by local calendar day,
// so every student sees the same line and it changes at midnight.
func (a *App) quoteOfToday() *Quote {
	quotes := make([]Quote, 0, len(seedQuotes))
	a.DB.Order("id asc").Find(&quotes)
	if len(quotes) == 0 {
		return nil
	}
	return &quotes[appNow().YearDay()%len(quotes)]
}

// ---------------------------------------------------------------------------
// Announcements
// ---------------------------------------------------------------------------

func validAnnouncementStatus(s string) bool {
	return s == "draft" || s == "published" || s == "archived"
}

// announcements serves the published board to any signed-in user: pinned
// first, newest next.
func (a *App) announcements(c *gin.Context) {
	limit := 10
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	rows := make([]Announcement, 0)
	a.DB.Where("status = ?", "published").Order("pinned desc, created_at desc").Limit(limit).Find(&rows)
	c.JSON(200, rows)
}

func (a *App) listAnnouncements(c *gin.Context) {
	rows := make([]Announcement, 0)
	a.DB.Order("pinned desc, created_at desc").Limit(200).Find(&rows)
	c.JSON(200, rows)
}

func (a *App) createAnnouncement(c *gin.Context) {
	var req struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Pinned *bool  `json:"pinned"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(400, gin.H{"error": "title is required"})
		return
	}
	if len(req.Body) > maxAnnouncementBody {
		c.JSON(400, gin.H{"error": "body exceeds the 8000-character limit"})
		return
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	if !validAnnouncementStatus(status) {
		c.JSON(400, gin.H{"error": "invalid status"})
		return
	}
	pinned := req.Pinned != nil && *req.Pinned
	announcement := Announcement{
		ID:         NewID("ann"),
		Title:      strings.TrimSpace(req.Title),
		Body:       req.Body,
		Pinned:     pinned,
		Status:     status,
		CreatedBy:  c.GetString("userID"),
		AuthorName: a.displayNameFor(c.GetString("userID")),
	}
	if err := a.DB.Create(&announcement).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not create announcement"})
		return
	}
	c.JSON(200, announcement)
}

func (a *App) updateAnnouncement(c *gin.Context) {
	var announcement Announcement
	if err := a.DB.First(&announcement, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Title  *string `json:"title"`
		Body   *string `json:"body"`
		Pinned *bool   `json:"pinned"`
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			c.JSON(400, gin.H{"error": "title cannot be empty"})
			return
		}
		announcement.Title = strings.TrimSpace(*req.Title)
	}
	if req.Body != nil {
		if len(*req.Body) > maxAnnouncementBody {
			c.JSON(400, gin.H{"error": "body exceeds the 8000-character limit"})
			return
		}
		announcement.Body = *req.Body
	}
	if req.Pinned != nil {
		announcement.Pinned = *req.Pinned
	}
	if req.Status != nil {
		if !validAnnouncementStatus(*req.Status) {
			c.JSON(400, gin.H{"error": "invalid status"})
			return
		}
		announcement.Status = *req.Status
	}
	if err := a.DB.Save(&announcement).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not save announcement"})
		return
	}
	c.JSON(200, announcement)
}

func (a *App) deleteAnnouncement(c *gin.Context) {
	id := c.Param("id")
	var announcement Announcement
	if err := a.DB.First(&announcement, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	if err := a.DB.Delete(&Announcement{}, "id = ?", id).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not delete announcement"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (a *App) displayNameFor(userID string) string {
	var user User
	if err := a.DB.First(&user, "id = ?", userID).Error; err != nil {
		return ""
	}
	return user.Name
}

// ---------------------------------------------------------------------------
// Calendar
// ---------------------------------------------------------------------------

func validEventKind(kind string) bool {
	return kind == "assignment" || kind == "assessment" || kind == "quiz" || kind == "holiday" || kind == "event"
}

func validLocalDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.ParseInLocation("2006-01-02", s, appTimeLocation())
	return err == nil
}

func validLocalTime(s string) bool {
	if len(s) != 5 {
		return false
	}
	_, err := time.Parse("15:04", s)
	return err == nil
}

// monthRange returns the first and last calendar date of "YYYY-MM".
func monthRange(month string) (string, string, bool) {
	first, err := time.ParseInLocation("2006-01", strings.TrimSpace(month), appTimeLocation())
	if err != nil {
		return "", "", false
	}
	return first.Format("2006-01-02"), first.AddDate(0, 1, -1).Format("2006-01-02"), true
}

type calendarPayload struct {
	Title   string  `json:"title"`
	Date    string  `json:"date"`
	EndDate *string `json:"endDate"`
	Time    *string `json:"time"`
	Kind    string  `json:"kind"`
	Note    string  `json:"note"`
}

// validateCalendarPayload normalizes and checks a create/update payload; the
// returned string is a client error message, "" means valid.
func validateCalendarPayload(req calendarPayload) string {
	if strings.TrimSpace(req.Title) == "" {
		return "title is required"
	}
	if !validLocalDate(req.Date) {
		return "date must be YYYY-MM-DD"
	}
	if req.EndDate != nil && *req.EndDate != "" {
		if !validLocalDate(*req.EndDate) {
			return "endDate must be YYYY-MM-DD"
		}
		if *req.EndDate < req.Date {
			return "endDate cannot be before date"
		}
	}
	if req.Time != nil && *req.Time != "" && !validLocalTime(*req.Time) {
		return "time must be HH:MM"
	}
	if !validEventKind(req.Kind) {
		return "kind must be assignment, assessment, quiz, holiday or event"
	}
	if len(req.Note) > maxCalendarNote {
		return "note exceeds the 2000-character limit"
	}
	return ""
}

func (req *calendarPayload) normalize() {
	req.Title = strings.TrimSpace(req.Title)
	if req.EndDate != nil && *req.EndDate == "" {
		req.EndDate = nil
	}
	if req.Time != nil && *req.Time == "" {
		req.Time = nil
	}
}

// calendarEvents serves one month of events (overlapping [monthStart,
// monthEnd], so multi-day entries straddling borders are included). Defaults
// to the current local month.
func (a *App) calendarEvents(c *gin.Context) {
	start, end, ok := monthRange(c.Query("month"))
	if !ok {
		start, end, _ = monthRange(appNow().Format("2006-01"))
	}
	events := make([]CalendarEvent, 0)
	a.DB.Where("date <= ? and (end_date is null or end_date >= ?)", end, start).
		Order("date asc, time asc, created_at asc").
		Limit(200).
		Find(&events)
	c.JSON(200, events)
}

// listCalendarEvents is the staff view: everything from two months back
// onwards (?all=1 drops the floor for cleanup of old entries).
func (a *App) listCalendarEvents(c *gin.Context) {
	events := make([]CalendarEvent, 0)
	q := a.DB.Order("date asc, time asc").Limit(500)
	if c.Query("all") == "" {
		q = q.Where("date >= ?", appNow().AddDate(0, -2, 0).Format("2006-01-02"))
	}
	q.Find(&events)
	c.JSON(200, events)
}

func (a *App) createCalendarEvent(c *gin.Context) {
	var req calendarPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	req.normalize()
	if msg := validateCalendarPayload(req); msg != "" {
		c.JSON(400, gin.H{"error": msg})
		return
	}
	event := CalendarEvent{ID: NewID("cal"), Title: req.Title, Date: req.Date, EndDate: req.EndDate, Time: req.Time, Kind: req.Kind, Note: req.Note, CreatedBy: c.GetString("userID")}
	if err := a.DB.Create(&event).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not create event"})
		return
	}
	c.JSON(200, event)
}

func (a *App) updateCalendarEvent(c *gin.Context) {
	var event CalendarEvent
	if err := a.DB.First(&event, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req calendarPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	req.normalize()
	if msg := validateCalendarPayload(req); msg != "" {
		c.JSON(400, gin.H{"error": msg})
		return
	}
	event.Title = req.Title
	event.Date = req.Date
	event.EndDate = req.EndDate
	event.Time = req.Time
	event.Kind = req.Kind
	event.Note = req.Note
	if err := a.DB.Save(&event).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not save event"})
		return
	}
	c.JSON(200, event)
}

func (a *App) deleteCalendarEvent(c *gin.Context) {
	id := c.Param("id")
	var event CalendarEvent
	if err := a.DB.First(&event, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	if err := a.DB.Delete(&CalendarEvent{}, "id = ?", id).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not delete event"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ---------------------------------------------------------------------------
// Concept stars
// ---------------------------------------------------------------------------

func (a *App) starConcept(c *gin.Context) {
	var req struct {
		Starred *bool `json:"starred"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Starred == nil {
		c.JSON(400, gin.H{"error": "starred (bool) is required"})
		return
	}
	state, err := a.stateFor(c.GetString("userID"), c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	state.Starred = *req.Starred
	a.DB.Save(&state)
	c.JSON(200, state)
}

type starredConceptRow struct {
	ConceptID  string `json:"conceptId"`
	Term       string `json:"term"`
	UnitTitle  string `json:"unitTitle"`
	TopicTitle string `json:"topicTitle"`
	Status     string `json:"status"`
}

// dashboardStarred lists the concepts the user starred, newest activity
// first, for the dashboard collection card.
func (a *App) dashboardStarred(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	rows := make([]starredConceptRow, 0)
	a.DB.Raw(`
		select c.id as concept_id, c.term as term, u.title as unit_title,
		       t.title as topic_title, s.status as status
		from user_concept_states s
		join concepts c on c.id = s.concept_id
		join units u on u.id = c.unit_id
		join topics t on t.id = c.topic_id
		where s.user_id = ? and s.starred = 1
		order by s.updated_at desc
		limit 8
	`, userID).Scan(&rows)
	c.JSON(200, rows)
}

type recentConceptRow struct {
	ConceptID  string    `json:"conceptId"`
	Term       string    `json:"term"`
	UnitTitle  string    `json:"unitTitle"`
	TopicTitle string    `json:"topicTitle"`
	Response   string    `json:"response"`
	LastAt     time.Time `json:"lastAt"`
}

// dashboardRecent lists the most recently reviewed concepts — one row per
// concept, latest event wins — for the dashboard recent-activity panel.
func (a *App) dashboardRecent(c *gin.Context) {
	userID := c.GetString("userID")
	rows := make([]recentConceptRow, 0)
	a.DB.Raw(`
		select e.concept_id, c.term, u.title as unit_title, t.title as topic_title,
		       e.response, e.created_at as last_at
		from review_events e
		join (
			select concept_id, max(created_at) as latest
			from review_events
			where user_id = ?
			group by concept_id
		) latest on latest.concept_id = e.concept_id and latest.latest = e.created_at
		join concepts c on c.id = e.concept_id
		join units u on u.id = c.unit_id
		join topics t on t.id = c.topic_id
		where e.user_id = ?
		group by e.concept_id
		order by last_at desc
		limit 8
	`, userID, userID).Scan(&rows)
	c.JSON(200, rows)
}
