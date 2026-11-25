package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(rebuildHardwareSchema, nil)
}

// rebuildHardwareSchema replaces the GPU-era hardware collections with a
// data-driven structure: hardware types, field definitions and table
// definitions live in hw_types/hw_fields/hw_tables, so new kinds of hardware
// can be added without code changes. The legacy collections were already
// emptied by clearHardwareData, so dropping them loses no records.
func rebuildHardwareSchema(app core.App) error {
	for _, name := range []string{"hardware_media", "hardware_options", "hardware"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		if err := app.Delete(collection); err != nil {
			return err
		}
	}

	types := core.NewBaseCollection("hw_types")
	types.Fields.Add(&core.TextField{Name: "key", Required: true})
	types.Fields.Add(&core.TextField{Name: "label", Required: true})
	types.Fields.Add(&core.TextField{Name: "name_field"})
	types.Fields.Add(&core.TextField{Name: "subtitle_fields"})
	types.Fields.Add(&core.NumberField{Name: "sort_order"})
	types.Indexes = append(types.Indexes, "CREATE UNIQUE INDEX idx_hw_types_key ON hw_types (key)")
	if err := app.Save(types); err != nil {
		return err
	}

	fields := core.NewBaseCollection("hw_fields")
	fields.Fields.Add(&core.RelationField{Name: "type", CollectionId: types.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
	fields.Fields.Add(&core.TextField{Name: "group", Required: true})
	fields.Fields.Add(&core.TextField{Name: "key", Required: true})
	fields.Fields.Add(&core.TextField{Name: "label", Required: true})
	fields.Fields.Add(&core.NumberField{Name: "sort_order"})
	if err := app.Save(fields); err != nil {
		return err
	}

	tables := core.NewBaseCollection("hw_tables")
	tables.Fields.Add(&core.RelationField{Name: "type", CollectionId: types.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
	tables.Fields.Add(&core.TextField{Name: "key", Required: true})
	tables.Fields.Add(&core.TextField{Name: "title", Required: true})
	tables.Fields.Add(&core.JSONField{Name: "columns", MaxSize: 1<<53 - 1})
	tables.Fields.Add(&core.NumberField{Name: "sort_order"})
	if err := app.Save(tables); err != nil {
		return err
	}

	hardware := core.NewBaseCollection("hardware")
	hardware.Fields.Add(&core.TextField{Name: "kind", Required: true})
	hardware.Fields.Add(&core.TextField{Name: "name", Max: 1<<31 - 1})
	hardware.Fields.Add(&core.TextField{Name: "brand", Max: 1<<31 - 1})
	hardware.Fields.Add(&core.TextField{Name: "subtitle", Max: 1<<31 - 1})
	hardware.Fields.Add(&core.NumberField{Name: "sort_order"})
	hardware.Fields.Add(&core.JSONField{Name: "data", MaxSize: 1<<53 - 1})
	if err := app.Save(hardware); err != nil {
		return err
	}

	media := core.NewBaseCollection("hardware_media")
	media.Fields.Add(&core.RelationField{Name: "hardware", CollectionId: hardware.Id, MaxSelect: 1, Required: true, CascadeDelete: true})
	media.Fields.Add(&core.JSONField{Name: "paths", MaxSize: 1<<53 - 1})
	media.Indexes = append(media.Indexes, "CREATE UNIQUE INDEX idx_hardware_media_hardware ON hardware_media (hardware)")
	if err := app.Save(media); err != nil {
		return err
	}

	options := core.NewBaseCollection("hardware_options")
	options.Fields.Add(&core.TextField{Name: "kind", Max: 1<<31 - 1})
	options.Fields.Add(&core.TextField{Name: "value", Max: 1<<31 - 1})
	options.Fields.Add(&core.NumberField{Name: "sort_order"})
	options.Indexes = append(options.Indexes, "CREATE UNIQUE INDEX idx_hardware_options_kind_value ON hardware_options (kind, value)")
	if err := app.Save(options); err != nil {
		return err
	}

	return seedHardwareSchema(app, types, fields, tables)
}

type hwFieldDef struct{ key, label string }
type hwGroupDef struct {
	title  string
	fields []hwFieldDef
}
type hwTableDef struct {
	key, title string
	columns    []hwFieldDef
}
type hwTypeDef struct {
	key, label, nameField, subtitleFields string
	groups                               []hwGroupDef
	tables                               []hwTableDef
}

// seedHardwareSchema migrates the hardware field definitions that were
// previously hardcoded in the frontend into hw_types/hw_fields/hw_tables.
func seedHardwareSchema(app core.App, types, fields, tables *core.Collection) error {
	gameColumns := []hwFieldDef{{"game", "游戏"}, {"resolution", "分辨率"}, {"quality", "画质"}, {"ray_tracing", "光线追踪"}, {"render_mode", "渲染模式"}, {"upscale_tech", "超分技术"}, {"upscale_quality", "超分档位"}, {"frame_gen", "帧生成"}, {"avg_fps", "平均帧"}, {"one_percent_low", "1% Low"}, {"min_fps", "最低帧数"}, {"gpu_temp", "显卡温度"}, {"gpu_usage", "GPU占用 (%)"}, {"cpu_usage", "CPU占用 (%)"}, {"ram_usage", "内存占用"}, {"vram_usage", "显存占用"}, {"power", "功耗"}, {"note", "备注"}}
	itemGameColumns := []hwFieldDef{{"game", "游戏"}, {"resolution", "分辨率"}, {"quality", "画质"}, {"ray_tracing", "光线追踪"}, {"avg_fps", "平均帧"}, {"one_percent_low", "1% Low"}, {"gpu_temp", "显卡温度"}, {"gpu_usage", "GPU占用 (%)"}, {"ram_usage", "内存占用"}, {"vram_usage", "显存占用"}, {"note", "备注"}}
	codecColumns := []hwFieldDef{{"codec", "编码器"}, {"resolution", "分辨率"}, {"time_sec", "耗时"}, {"power", "功耗"}, {"gpu_usage", "GPU占用"}, {"cpu_usage", "CPU占用"}, {"note", "备注"}}
	mark3d := []hwFieldDef{{"solar_bay", "Solar Bay"}, {"steel_nomad", "Steel Nomad"}, {"steel_nomad_light", "Steel Nomad Light"}, {"speed_way", "Speed Way"}, {"port_royal", "Port Royal"}, {"time_spy", "Time Spy"}, {"fire_strike", "Fire Strike"}, {"dx_raytracing", "DX 光追 (FPS)"}, {"pcie_bandwidth", "PCIE 带宽 (GB/s)"}}

	defs := []hwTypeDef{
		{
			key: "gpu", label: "显卡", nameField: "model", subtitleFields: "brand,vram_size",
			groups: []hwGroupDef{
				{"基本参数", []hwFieldDef{{"model", "显卡型号"}, {"brand", "品牌"}, {"vram_size", "显存容量"}, {"vram_type", "显存类型"}, {"vram_clock", "显存频率"}, {"gpu_clock", "核心频率"}, {"tgp", "TGP (W)"}, {"actual_power", "实测功耗 (W)"}, {"driver_version", "驱动版本"}, {"test_date", "测试日期"}, {"test_platform", "测试平台"}, {"ambient_temp", "环境温度"}}},
				{"散热与功耗", []hwFieldDef{{"default_fan_temp", "默认转速满载温度"}, {"max_fan_temp", "满转速满载温度"}, {"temp_45db", "45分贝满载温度"}, {"core_temp", "满载核心温度"}, {"hotspot_temp", "热点温度"}, {"vram_temp", "显存温度"}, {"default_fan_rpm", "默认转速 (RPM)"}, {"max_fan_rpm", "满转速 (RPM)"}, {"rpm_45db", "45分贝转速"}, {"full_load_power", "满载功耗 (W)"}, {"power_45db", "45分贝功耗 (W)"}, {"temp_limit", "温度墙"}, {"power_limit", "功耗墙"}, {"throttling", "降频情况"}}},
				{"3DMark", mark3d},
				{"SolidWorks (SW)", []hwFieldDef{{"graphics", "图形"}, {"processor", "处理器"}, {"io", "输入/输出"}, {"realview", "RealView 性能"}, {"simulation", "模拟"}}},
				{"Blender", []hwFieldDef{{"monster", "Monster"}, {"junkshop", "Junkshop"}, {"classroom", "Classroom"}}},
				{"备注", []hwFieldDef{{"note", "备注"}}},
			},
			tables: []hwTableDef{
				{"productivity", "生产力测试", []hwFieldDef{{"name", "项目"}, {"score", "得分"}, {"unit", "单位"}, {"note", "备注"}}},
				{"games", "游戏测试", gameColumns},
				{"encoding", "编码测试", codecColumns},
				{"decoding", "解码测试", codecColumns},
			},
		},
		{
			key: "cpu", label: "CPU", nameField: "model", subtitleFields: "cores,boost_clock",
			groups: []hwGroupDef{
				{"基本参数", []hwFieldDef{{"model", "型号"}, {"brand", "品牌"}, {"socket", "插槽"}, {"cores", "核心数"}, {"threads", "线程数"}, {"p_cores", "大核数"}, {"e_cores", "小核数"}, {"base_clock", "基础频率"}, {"boost_clock", "加速频率"}, {"l3_cache", "L3 缓存"}, {"tdp", "TDP (W)"}, {"process", "制程"}, {"igpu", "核显"}, {"memory", "内存支持"}, {"test_platform", "测试平台"}, {"bios", "BIOS / 微码"}, {"test_date", "测试日期"}, {"ambient_temp", "环境温度"}}},
				{"跑分", []hwFieldDef{{"cpuz_single", "CPU-Z 单核"}, {"cpuz_multi", "CPU-Z 多核"}, {"cb23_single", "Cinebench R23 单核"}, {"cb23_multi", "Cinebench R23 多核"}, {"cb24_single", "Cinebench 2024 单核"}, {"cb24_multi", "Cinebench 2024 多核"}, {"geekbench_single", "Geekbench 单核"}, {"geekbench_multi", "Geekbench 多核"}, {"timespy_cpu", "3DMark CPU Profile"}, {"cpu_profile_1t", "CPU Profile 1T"}, {"passmark", "PassMark"}, {"ycruncher", "y-cruncher"}}},
				{"功耗与温度", []hwFieldDef{{"idle_power", "待机功耗 (W)"}, {"full_power", "满载功耗 (W)"}, {"package_power", "封装功耗 (W)"}, {"temp_idle", "待机温度 (℃)"}, {"temp_full", "满载温度 (℃)"}, {"temp_45db", "45分贝温度 (℃)"}, {"temp_limit", "温度墙"}, {"power_limit", "功耗墙"}, {"throttling", "降频情况"}}},
				{"备注", []hwFieldDef{{"note", "备注"}}},
			},
			tables: []hwTableDef{
				{"power_steps", "功耗档位测试", []hwFieldDef{{"power", "功耗 (W)"}, {"multi", "多核分数"}, {"single", "单核分数"}, {"temp", "温度"}, {"note", "说明"}}},
				{"games", "游戏测试", itemGameColumns},
			},
		},
		{
			key: "device", label: "设备", nameField: "name", subtitleFields: "type,cpu,gpu",
			groups: []hwGroupDef{
				{"基本信息", []hwFieldDef{{"name", "设备名称"}, {"type", "类型"}, {"brand", "品牌"}, {"model", "型号"}, {"cpu", "CPU"}, {"gpu", "显卡"}, {"ram", "内存"}, {"storage", "存储"}, {"os", "系统"}}},
				{"性能测试", append(append([]hwFieldDef{}, mark3d[:8]...), hwFieldDef{"cinebench_multi", "Cinebench 多核"}, hwFieldDef{"cinebench_single", "Cinebench 单核"}, hwFieldDef{"geekbench_multi", "Geekbench 多核"}, hwFieldDef{"pcmark", "PCMark"})},
				{"功耗、温度与续航", []hwFieldDef{{"idle_power", "待机功耗 (W)"}, {"full_power", "满载功耗 (W)"}, {"temp_idle", "待机温度 (℃)"}, {"temp_full", "满载温度 (℃)"}, {"fan_noise", "满载噪音 (dB)"}, {"battery_life", "续航 (小时)"}}},
				{"备注", []hwFieldDef{{"note", "备注"}}},
			},
			tables: []hwTableDef{
				{"games", "游戏测试", itemGameColumns},
			},
		},
	}

	for i, def := range defs {
		typeRecord := core.NewRecord(types)
		typeRecord.Set("key", def.key)
		typeRecord.Set("label", def.label)
		typeRecord.Set("name_field", def.nameField)
		typeRecord.Set("subtitle_fields", def.subtitleFields)
		typeRecord.Set("sort_order", (i+1)*10)
		if err := app.Save(typeRecord); err != nil {
			return err
		}
		order := 0
		for _, group := range def.groups {
			for _, field := range group.fields {
				order++
				record := core.NewRecord(fields)
				record.Set("type", typeRecord.Id)
				record.Set("group", group.title)
				record.Set("key", field.key)
				record.Set("label", field.label)
				record.Set("sort_order", order)
				if err := app.Save(record); err != nil {
					return err
				}
			}
		}
		for j, table := range def.tables {
			columns := make([]map[string]string, 0, len(table.columns))
			for _, column := range table.columns {
				columns = append(columns, map[string]string{"key": column.key, "label": column.label})
			}
			record := core.NewRecord(tables)
			record.Set("type", typeRecord.Id)
			record.Set("key", table.key)
			record.Set("title", table.title)
			record.Set("columns", columns)
			record.Set("sort_order", (j+1)*10)
			if err := app.Save(record); err != nil {
				return err
			}
		}
	}
	return nil
}
