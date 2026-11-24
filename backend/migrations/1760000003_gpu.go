package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(func(app core.App) error {
		_, err := app.DB().NewQuery(`
CREATE TABLE gpus (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  model TEXT NOT NULL DEFAULT '',
  brand TEXT NOT NULL DEFAULT '',
  vram_size TEXT NOT NULL DEFAULT '',
  vram_type TEXT NOT NULL DEFAULT '',
  vram_clock TEXT NOT NULL DEFAULT '',
  gpu_clock TEXT NOT NULL DEFAULT '',
  tgp TEXT NOT NULL DEFAULT '',
  actual_power TEXT NOT NULL DEFAULT '',
  driver_version TEXT NOT NULL DEFAULT '',
  test_date TEXT NOT NULL DEFAULT '',
  test_platform TEXT NOT NULL DEFAULT '',
  ambient_temp TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  related_media TEXT NOT NULL DEFAULT '[]'
);
CREATE TABLE cooling (
  gpu_id INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  default_fan_temp TEXT NOT NULL DEFAULT '',
  max_fan_temp TEXT NOT NULL DEFAULT '',
  temp_45db TEXT NOT NULL DEFAULT '',
  core_temp TEXT NOT NULL DEFAULT '',
  hotspot_temp TEXT NOT NULL DEFAULT '',
  vram_temp TEXT NOT NULL DEFAULT '',
  default_fan_rpm TEXT NOT NULL DEFAULT '',
  max_fan_rpm TEXT NOT NULL DEFAULT '',
  rpm_45db TEXT NOT NULL DEFAULT '',
  full_load_power TEXT NOT NULL DEFAULT '',
  power_45db TEXT NOT NULL DEFAULT '',
  temp_limit TEXT NOT NULL DEFAULT '',
  power_limit TEXT NOT NULL DEFAULT '',
  throttling TEXT NOT NULL DEFAULT ''
);
CREATE TABLE mark3d (
  gpu_id INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  solar_bay TEXT NOT NULL DEFAULT '',
  steel_nomad TEXT NOT NULL DEFAULT '',
  steel_nomad_light TEXT NOT NULL DEFAULT '',
  speed_way TEXT NOT NULL DEFAULT '',
  port_royal TEXT NOT NULL DEFAULT '',
  time_spy TEXT NOT NULL DEFAULT '',
  fire_strike TEXT NOT NULL DEFAULT '',
  dx_raytracing TEXT NOT NULL DEFAULT '',
  pcie_bandwidth TEXT NOT NULL DEFAULT ''
);
CREATE TABLE blender (
  gpu_id INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  monster TEXT NOT NULL DEFAULT '',
  junkshop TEXT NOT NULL DEFAULT '',
  classroom TEXT NOT NULL DEFAULT ''
);
CREATE TABLE sw (
  gpu_id INTEGER PRIMARY KEY REFERENCES gpus(id) ON DELETE CASCADE,
  graphics TEXT NOT NULL DEFAULT '',
  processor TEXT NOT NULL DEFAULT '',
  io TEXT NOT NULL DEFAULT '',
  realview TEXT NOT NULL DEFAULT '',
  simulation TEXT NOT NULL DEFAULT ''
);
CREATE TABLE productivity (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT '',
  score TEXT NOT NULL DEFAULT '',
  unit TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE games (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  game TEXT NOT NULL DEFAULT '',
  resolution TEXT NOT NULL DEFAULT '',
  quality TEXT NOT NULL DEFAULT '',
  render_mode TEXT NOT NULL DEFAULT '',
  upscale_tech TEXT NOT NULL DEFAULT '',
  upscale_quality TEXT NOT NULL DEFAULT '',
  frame_gen TEXT NOT NULL DEFAULT '',
  avg_fps TEXT NOT NULL DEFAULT '',
  min_fps TEXT NOT NULL DEFAULT '',
  one_percent_low TEXT NOT NULL DEFAULT '',
  ray_tracing TEXT NOT NULL DEFAULT '',
  gpu_temp TEXT NOT NULL DEFAULT '',
  gpu_usage TEXT NOT NULL DEFAULT '',
  cpu_usage TEXT NOT NULL DEFAULT '',
  ram_usage TEXT NOT NULL DEFAULT '',
  vram_usage TEXT NOT NULL DEFAULT '',
  power TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE codec_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  gpu_id INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  codec TEXT NOT NULL DEFAULT '',
  resolution TEXT NOT NULL DEFAULT '',
  time_sec TEXT NOT NULL DEFAULT '',
  power TEXT NOT NULL DEFAULT '',
  gpu_usage TEXT NOT NULL DEFAULT '',
  cpu_usage TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_productivity_gpu ON productivity(gpu_id);
CREATE INDEX idx_games_gpu ON games(gpu_id);
CREATE INDEX idx_codec_gpu ON codec_runs(gpu_id, kind);
CREATE TABLE items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  brand TEXT NOT NULL DEFAULT '',
  subtitle TEXT NOT NULL DEFAULT '',
  fields TEXT NOT NULL DEFAULT '{}',
  rows TEXT NOT NULL DEFAULT '{}',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_items_kind ON items(kind, sort_order, id);
CREATE TABLE gpu_custom (
  gpu_id INTEGER NOT NULL REFERENCES gpus(id) ON DELETE CASCADE,
  section TEXT NOT NULL,
  field_key TEXT NOT NULL,
  label TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  hidden INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (gpu_id, section, field_key)
);
CREATE INDEX idx_gpu_custom ON gpu_custom(gpu_id, section, sort_order);
CREATE TABLE option_values (
  kind TEXT NOT NULL,
  value TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (kind, value)
);
CREATE TABLE hpahub_gpu_import (
  id INTEGER PRIMARY KEY CHECK (id = 1)
);
`).Execute()
		return err
	}, nil)
}
