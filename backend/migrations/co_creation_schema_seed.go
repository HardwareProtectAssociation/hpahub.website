package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

type coIdentitySeed struct {
	category    string
	name        string
	description string
}

type coSkillSeed struct {
	name     string
	category string
}

type coCareerLevelSeed struct {
	number      int
	minimumXP   int
	displayName string
}

// coIdentitySeeds is the collaboration-identity dictionary from §4 of the plan.
// The plan labels the concrete entries as a suggestion, so the rows stay
// editable (enabled/xp_enabled) once the platform is running.
var coIdentitySeeds = []coIdentitySeed{
	{"main", "软件开发者", "网站、应用、程序、算法、游戏开发"},
	{"main", "电子工程师", "电路设计、PCB、电子调试"},
	{"main", "嵌入式开发者", "单片机、固件、设备控制"},
	{"main", "机械与结构工程师", "机械设计、结构、运动机构"},
	{"main", "产品与工业设计师", "产品形态、功能、人机交互"},
	{"main", "手工制作者", "木工、金工、皮具、陶艺、缝纫"},
	{"main", "数字制造者", "建模、3D 打印、激光切割、数控加工"},
	{"main", "科学研究者", "研究方案、理论分析、提出假设"},
	{"main", "实验与测试研究者", "实验、测量、验证、数据分析"},
	{"main", "视觉与艺术创作者", "绘画、平面、雕塑、动画"},
	{"main", "内容创作者", "写作、摄影、视频、音乐、声音"},
	{"main", "建筑与空间设计者", "建筑、室内、展陈、空间装置"},
	{"main", "教育内容开发者", "课程、教程、教学工具"},
	{"main", "项目策划者", "提出方案、设计内容、组织成果"},
	{"support", "资金赞助者", "提供资金"},
	{"support", "工具提供者", "提供或借用工具设备"},
	{"support", "材料提供者", "零件、原料、耗材"},
	{"support", "场地提供者", "工作室、实验室、活动和展示空间"},
	{"support", "技术顾问", "答疑、方案评审、指导"},
	{"support", "测试与体验者", "试用、找问题、体验反馈"},
	{"support", "项目管理者", "计划、进度、任务协调"},
	{"support", "协作协调员", "成员对接、沟通需求"},
	{"support", "资料整理者", "资料搜集、参考来源、过程记录"},
	{"support", "文档与教程编写者", "说明书、指南、制作教程"},
	{"support", "翻译与本地化协作者", "资料、字幕、多语言内容"},
	{"support", "传播与推广者", "宣传、项目介绍、社群运营"},
	{"support", "采购与后勤协作者", "供应商、采购、运输、物资"},
	{"support", "财务与预算协作者", "预算、支出、赞助使用记录"},
	{"support", "法律与知识产权顾问", "合作约定、授权、知识产权"},
	{"support", "心理辅导与情绪支持者", "倾听、鼓励、压力与沟通支持"},
	{"support", "社区维护者", "引导讨论、整理反馈、交流秩序"},
	{"support", "资源联络者", "人才、机构、合作伙伴、赞助渠道"},
}

// coSkillSeeds is a starter skill dictionary for search and matching. Users
// can still enter only the skills listed here; extend the rows as needed.
var coSkillSeeds = []coSkillSeed{
	{"PCB 设计", "电子"},
	{"电路设计", "电子"},
	{"模拟电路", "电子"},
	{"数字电路", "电子"},
	{"固件开发", "电子"},
	{"单片机", "电子"},
	{"嵌入式 C", "电子"},
	{"机械结构设计", "机械"},
	{"CAD 建模", "机械"},
	{"3D 打印", "制造"},
	{"激光切割", "制造"},
	{"数控加工", "制造"},
	{"木工", "制造"},
	{"金属加工", "制造"},
	{"皮革工艺", "制造"},
	{"陶艺", "制造"},
	{"缝纫", "制造"},
	{"平面设计", "设计"},
	{"3D 渲染", "设计"},
	{"摄影", "内容"},
	{"视频剪辑", "内容"},
	{"写作", "内容"},
	{"教程编写", "内容"},
	{"翻译", "内容"},
	{"项目管理", "管理"},
	{"预算管理", "管理"},
	{"采购与物流", "管理"},
	{"法务与知识产权", "法务"},
	{"心理支持", "支持"},
	{"社区运营", "支持"},
	{"资源对接", "支持"},
}

// coCareerLevelSeeds is the example career curve from §15 (D18 still pending).
var coCareerLevelSeeds = []coCareerLevelSeed{
	{1, 0, "Lv.1"},
	{2, 100, "Lv.2"},
	{3, 300, "Lv.3"},
	{4, 600, "Lv.4"},
	{5, 1000, "Lv.5"},
}

// seedDictionaries inserts the identity/skill dictionaries and the demo career
// level thresholds. The plan requires an identity/skill dictionary and demo
// data; project and account demo data are separate, later tasks.
func (s *coCreation) seedDictionaries() error {
	identityIDs := make([]string, 0, len(coIdentitySeeds))
	for _, item := range coIdentitySeeds {
		record := core.NewRecord(s.identities)
		record.Set("category", item.category)
		record.Set("name", item.name)
		record.Set("description", item.description)
		record.Set("enabled", true)
		record.Set("xp_enabled", true)
		if err := s.app.Save(record); err != nil {
			return err
		}
		identityIDs = append(identityIDs, record.Id)
	}

	for _, item := range coSkillSeeds {
		record := core.NewRecord(s.skills)
		record.Set("name", item.name)
		record.Set("category", item.category)
		if err := s.app.Save(record); err != nil {
			return err
		}
	}

	for _, identityID := range identityIDs {
		for _, level := range coCareerLevelSeeds {
			record := core.NewRecord(s.thresholds)
			record.Set("career_identity_id", identityID)
			record.Set("level_number", level.number)
			record.Set("minimum_total_xp", level.minimumXP)
			record.Set("display_title", level.displayName)
			if err := s.app.Save(record); err != nil {
				return err
			}
		}
	}
	return nil
}
