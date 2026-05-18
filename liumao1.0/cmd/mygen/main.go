package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/12qwaszx3edc123/bksgpx/templates"
	_ "github.com/go-sql-driver/mysql"
)

const VERSION = "1.0.1"

type FieldInfo struct {
	ColumnName string
	GoName     string
	GoType     string
	ProtoType  string
	ProtoNum   int
}

type ModuleInfo struct {
	Name      string
	UpperName string
	LowerName string
	Port      int
	Fields    []FieldInfo
}

type Config struct {
	Name       string
	BFF        string
	Modules    string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	Template   string
}

type ProjectData struct {
	AppName    string
	BffDirName string
	Modules    []ModuleInfo
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

type ModuleData struct {
	AppName    string
	BffDirName string
	Module     ModuleInfo
}

func main() {
	cfg := &Config{}
	flag.StringVar(&cfg.Name, "name", "liumao", "project name")
	flag.StringVar(&cfg.BFF, "bff", "h5", "bff type: h5, web, applet, app")
	flag.StringVar(&cfg.Modules, "modules", "", "comma-separated modules, e.g. user,order,doctor")
	flag.StringVar(&cfg.DBHost, "db-host", "127.0.0.1", "database host")
	flag.StringVar(&cfg.DBPort, "db-port", "3306", "database port")
	flag.StringVar(&cfg.DBUser, "db-user", "root", "database user")
	flag.StringVar(&cfg.DBPassword, "db-password", "root", "database password")
	flag.StringVar(&cfg.DBName, "db-name", "", "database name")
	flag.StringVar(&cfg.Template, "template", "", "external template directory (default: embedded)")
	flag.Parse()

	if !filepath.IsAbs(cfg.Name) {
		absPath, err := filepath.Abs(cfg.Name)
		if err == nil {
			cfg.Name = absPath
		}
	}

	modules := splitCSV(cfg.Modules)
	if len(modules) == 0 {
		log.Fatal("必须指定至少一个模块，使用 --modules 参数")
	}

	var moduleInfos []ModuleInfo
	for i, mod := range modules {
		moduleInfos = append(moduleInfos, ModuleInfo{
			Name:      mod,
			UpperName: capitalize(mod),
			LowerName: mod,
			Port:      50051 + i,
		})
	}

	// 连接数据库读取表字段
	if cfg.DBName != "" {
		db, err := connectDB(cfg)
		if err != nil {
			log.Printf("警告: 无法连接数据库 (%v)，将生成骨架代码\n", err)
		} else {
			defer db.Close()
			for i, mod := range moduleInfos {
				fields, err := queryTableFields(db, cfg.DBName, mod.LowerName)
				if err != nil {
					log.Printf("警告: 无法读取表 %s 的字段 (%v)，将生成骨架代码\n", mod.LowerName, err)
				} else if len(fields) == 0 {
					log.Printf("警告: 表 %s 不存在或没有业务字段，将生成骨架代码\n", mod.LowerName)
				} else {
					moduleInfos[i].Fields = fields
					fmt.Printf("表 %s: 读取到 %d 个字段\n", mod.LowerName, len(fields))
				}
			}
		}
	}

	projectData := ProjectData{
		AppName:    filepath.Base(cfg.Name),
		BffDirName: "bff" + toCamelCase(cfg.BFF),
		Modules:    moduleInfos,
		DBHost:     cfg.DBHost,
		DBPort:     cfg.DBPort,
		DBUser:     cfg.DBUser,
		DBPassword: cfg.DBPassword,
		DBName:     cfg.DBName,
	}

	// 初始化模板文件系统
	var tFS fs.FS
	if cfg.Template != "" {
		// 使用外部模板目录
		tFS = os.DirFS(cfg.Template)
		fmt.Printf("使用外部模板: %s\n", cfg.Template)
	} else {
		// 使用内嵌模板
		var err error
		tFS, err = fs.Sub(templates.Bks, "liumao")
		if err != nil {
			log.Fatalf("加载内嵌模板失败: %v", err)
		}
		fmt.Println("使用内嵌模板")
	}

	outputDir := cfg.Name

	fmt.Printf("mygen v%s\n", VERSION)
	fmt.Printf("生成项目: %s\n", outputDir)
	fmt.Printf("模块: %s\n", strings.Join(func() []string {
		var names []string
		for _, m := range moduleInfos {
			names = append(names, fmt.Sprintf("%s(:%d)", m.Name, m.Port))
		}
		return names
	}(), ", "))

	generateProjectFiles(projectData, outputDir, tFS)

	for _, mod := range moduleInfos {
		moduleData := ModuleData{
			AppName:    projectData.AppName,
			BffDirName: projectData.BffDirName,
			Module:     mod,
		}
		generateModuleFiles(moduleData, outputDir, tFS)
	}

	generateGoMod(projectData, outputDir)

	fmt.Println("\n完成!")
	fmt.Printf("项目已生成到: %s\n", outputDir)
	fmt.Println("\n后续步骤:")
	fmt.Println("1. 修改 common/config/config.yaml 中的空值配置")
	fmt.Println("2. 运行 protoc 生成 Go 代码")
	fmt.Println("3. 运行 go mod tidy 下载依赖")
}

func connectDB(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	return sql.Open("mysql", dsn)
}

func queryTableFields(db *sql.DB, dbName, tableName string) ([]FieldInfo, error) {
	query := `SELECT COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`
	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gormFields := map[string]bool{
		"id": true, "created_at": true, "updated_at": true, "deleted_at": true,
	}

	var fields []FieldInfo
	num := 1
	for rows.Next() {
		var columnName, dataType string
		if err := rows.Scan(&columnName, &dataType); err != nil {
			return nil, err
		}
		if gormFields[columnName] {
			continue
		}
		goType, protoType := mysqlTypeToGo(dataType)
		fields = append(fields, FieldInfo{
			ColumnName: columnName,
			GoName:     toCamelCase(columnName),
			GoType:     goType,
			ProtoType:  protoType,
			ProtoNum:   num,
		})
		num++
	}
	return fields, nil
}

func mysqlTypeToGo(mysqlType string) (string, string) {
	switch strings.ToLower(mysqlType) {
	case "varchar", "char", "text", "longtext", "mediumtext", "tinytext", "enum":
		return "string", "string"
	case "int", "integer", "tinyint", "smallint", "mediumint":
		return "int32", "int32"
	case "bigint":
		return "int64", "int64"
	case "float":
		return "float32", "float"
	case "double", "decimal":
		return "float64", "double"
	case "date", "datetime", "timestamp":
		return "string", "string"
	case "bit", "bool", "boolean":
		return "bool", "bool"
	default:
		return "string", "string"
	}
}

func generateProjectFiles(data ProjectData, outputDir string, tFS fs.FS) {
	processTemplate(tFS,
		"common/config/config.yaml.tmpl",
		filepath.Join(outputDir, "common", "config", "config.yaml"),
		data,
	)
	processTemplate(tFS,
		"common/config/config.go.tmpl",
		filepath.Join(outputDir, "common", "config", "config.go"),
		data,
	)
	processTemplate(tFS,
		"common/config/global.go.tmpl",
		filepath.Join(outputDir, "common", "config", "global.go"),
		data,
	)
	processTemplate(tFS,
		"common/init/init.go.tmpl",
		filepath.Join(outputDir, "common", "init", "init.go"),
		data,
	)
	processTemplate(tFS,
		"bff/cmd/main.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "basic", "cmd", "main.go"),
		data,
	)
	processTemplate(tFS,
		"bff/middlewares/middlewares.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "basic", "middlewares", "middlewares.go"),
		data,
	)
	processTemplate(tFS,
		"bff/router/router.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "basic", "router", "router.go"),
		data,
	)
	processTemplate(tFS,
		"bff/handler/api/upload.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "handler", "api", "upload.go"),
		data,
	)
	processTemplate(tFS,
		"bff/handler/request/upload.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "handler", "request", "upload.go"),
		data,
	)
	pkgFiles := []string{"jwt.go", "upload.go", "cart.go", "alipay.go", "sendsms.go", "ordersn.go"}
	for _, f := range pkgFiles {
		processTemplate(tFS,
			path.Join("pkg", f+".tmpl"),
			filepath.Join(outputDir, "pkg", f),
			data,
		)
	}
}

func generateModuleFiles(data ModuleData, outputDir string, tFS fs.FS) {
	mod := data.Module
	processTemplate(tFS,
		"proto/proto.tmpl",
		filepath.Join(outputDir, "proto", mod.LowerName, mod.LowerName+".proto"),
		data,
	)
	processTemplate(tFS,
		"common/model/model.go.tmpl",
		filepath.Join(outputDir, "common", "model", mod.LowerName+".go"),
		data,
	)
	processTemplate(tFS,
		"srv/cmd/main.go.tmpl",
		filepath.Join(outputDir, mod.LowerName+"-server", "basic", "cmd", "main.go"),
		data,
	)
	processTemplate(tFS,
		"srv/server/server.go.tmpl",
		filepath.Join(outputDir, mod.LowerName+"-server", "server", mod.LowerName+".go"),
		data,
	)
	processTemplate(tFS,
		"bff/handler/api/handler.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "handler", "api", mod.LowerName+".go"),
		data,
	)
	processTemplate(tFS,
		"bff/handler/request/request.go.tmpl",
		filepath.Join(outputDir, data.BffDirName, "handler", "request", mod.LowerName+".go"),
		data,
	)
}

func generateGoMod(data ProjectData, outputDir string) {
	goModContent := fmt.Sprintf(`module %s

go 1.26

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-gonic/gin v1.12.0
	github.com/olivere/elastic/v7 v7.0.32
	github.com/qiniu/go-sdk/v7 v7.26.12
	github.com/redis/go-redis/v9 v9.19.0
	github.com/smartwalle/alipay/v3 v3.2.29
	github.com/spf13/viper v1.21.0
	google.golang.org/grpc v1.81.0
	google.golang.org/protobuf v1.36.11
	gorm.io/driver/mysql v1.6.0
	gorm.io/gorm v1.31.1
)
`, data.AppName)

	os.MkdirAll(outputDir, 0755)
	err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		log.Printf("写入 go.mod 失败: %v", err)
	} else {
		fmt.Printf("生成: go.mod\n")
	}
}

func processTemplate(tFS fs.FS, srcTmpl, dstFile string, data interface{}) {
	tmplContent, err := fs.ReadFile(tFS, srcTmpl)
	if err != nil {
		log.Printf("跳过: 模板文件不存在 %s", srcTmpl)
		return
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New(path.Base(srcTmpl)).Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		log.Printf("解析模板失败 %s: %v", srcTmpl, err)
		return
	}

	os.MkdirAll(filepath.Dir(dstFile), 0755)
	outFile, err := os.Create(dstFile)
	if err != nil {
		log.Printf("创建文件失败 %s: %v", dstFile, err)
		return
	}
	defer outFile.Close()

	err = tmpl.Execute(outFile, data)
	if err != nil {
		log.Printf("执行模板失败 %s: %v", srcTmpl, err)
		return
	}

	fmt.Printf("生成: %s\n", dstFile)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func toCamelCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	var result string
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return result
}
