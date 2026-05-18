# bksgpx

Go 微服务脚手架生成工具 - 一键生成完整的微服务项目结构

## 安装

```bash
go install github.com/12qwaszx3edc123/bksgpx/cmd/mygen@latest
```

## 使用方法

```bash
mygen --name <项目名> --modules <模块列表> [其他参数]
```

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--name` | `bks` | 项目名称 |
| `--modules` | - | 模块列表，逗号分隔，如 `user,order,doctor` |
| `--bff` | `h5` | BFF 类型: `h5`, `web`, `applet`, `app` |
| `--db-host` | `127.0.0.1` | 数据库主机 |
| `--db-port` | `3306` | 数据库端口 |
| `--db-user` | `root` | 数据库用户名 |
| `--db-password` | `root` | 数据库密码 |
| `--db-name` | - | 数据库名称 |
| `--template` | 内嵌模板 | 外部模板目录路径 |

### 示例

```bash
# 生成名为 myshop 的项目，包含 user 和 order 模块
mygen --name myshop --modules user,order --db-name myshop_db

# 使用外部模板
mygen --name myproject --modules user --template /path/to/templates
```

## 生成的项目结构

```
<项目名>/
├── common/
│   ├── config/      # 配置管理
│   ├── init/        # 初始化
│   └── model/       # 数据模型
├── proto/           # Protobuf 定义
├── <module>-srv/    # 各模块的 gRPC 服务
├── bff<type>/       # BFF 层 (h5/web/applet/app)
└── pkg/             # 公共工具包
```

## 模板

默认使用内嵌模板，无需额外配置。如需自定义模板，可使用 `--template` 参数指定外部模板目录。

---

## ElasticSearch 操作

### ESAdd //ES添加

```go
// 同步 Elastic添加
m := map[string]interface{}{
	"id", "name", "image", "price", "stock",       // 菜品基本信息
	"merchant_id", "type", "month_sales",          // 商家和销量信息
	"rating", "is_new", "status",                  // 状态信息
}
_, err = config.Esc.Index().
	Index("commodity").              // 索引名称
	BodyJson(m).                     // 文档内容
	Do(context.Background())         // 执行
```

### ESList //ES列表

```go
// 分页
offset, size := pkg.Paginate(int(req.Page), int(req.Size))

// 布尔查询构建
boolQuery := elastic.NewBoolQuery()

// 名称模糊搜索
elastic.NewMatchQuery("name", req.Name).Must(boolQuery)

// ID精准搜索
elastic.NewTermQuery("merchant_id", req.MerchantId).Must(boolQuery)

// 价格区间查询
elastic.NewRangeQuery("price").Gte(req.MinPrice).Lte(req.MaxPrice).Must(boolQuery)

// 排序：格式 "字段_方向"，如 "Rating_Desc"、"price_Asc"
sortQuery := elastic.NewFieldSort("rating")       // 按评分排序
sortQuery := elastic.NewFieldSort("month_sales")  // 按月销量排序
sortQuery := elastic.NewFieldSort("price")        // 按价格排序
sortQuery.Desc()                                  // 倒序
sortQuery.Asc()                                   // 正序

// 执行搜索
do, err := config.Esc.Search().
	Index("commodity").              // 索引名称
	From(offset).                    // 偏移量
	Size(size).                      // 每页数量
	Query(boolQuery).                // 查询条件
	SortBy(sortQuery).               // 排序规则
	Do(context.Background())

// 总条数
total := do.Hits.TotalHits

// 结果解析
for _, hit := range do.Hits.Hits {
	json.Unmarshal(hit.Source, list)  // 反序列化每条记录
}
```

### ESGet //ES查询

```go
// 根据ID查询单条文档
do, err := config.Esc.Get().
	Index("commodity").              // 索引名称
	Id(strconv.Itoa(int(id))).       // 文档ID
	Do(context.Background())         // 执行

// 解析结果
if do.Found {
	json.Unmarshal(do.Source, &result)  // 反序列化文档内容
}
```

### ESUpdate //ES修改

```go
// 根据ID更新文档字段
m := map[string]interface{}{
	"name",   // 修改的字段
	"price",  // 修改的字段
	"stock",  // 修改的字段
}
_, err = config.Esc.Update().
	Index("commodity").              // 索引名称
	Id(strconv.Itoa(int(id))).       // 文档ID
	Doc(m).                          // 更新内容
	Do(context.Background())         // 执行
```

### ESDelete //ES删除

```go
// 根据ID删除文档
_, err = config.Esc.Delete().
	Index("commodity").              // 索引名称
	Id(strconv.Itoa(int(id))).       // 文档ID
	Do(context.Background())         // 执行
```

### ESHighlight //ES高亮搜索

```go
// 设置高亮
highlight := elastic.NewHighlight()
highlight.Field("name")                          // 高亮哪个字段
highlight.PreTags("<em>")                        // 前置标签
highlight.PostTags("</em>")                      // 后置标签

// 执行搜索
do, err := config.Esc.Search().
	Index("commodity").              // 索引名称
	Query(boolQuery).                // 查询条件
	Highlight(highlight).            // 添加高亮
	Do(context.Background())         // 执行

// 获取高亮结果
for _, hit := range do.Hits.Hits {
	if hit.Highlight != nil {
		highlightedName := hit.Highlight["name"]  // ["<em>感冒</em>灵颗粒"]
	}
}
```

---

## Redis Cart //Redis购物车

Key格式：`cart:{UserId}:{DrugId}`

### CartAdd //购物车添加/修改

```go
key := fmt.Sprintf("cart:%d:%d", UserId, DrugId)   // key格式
config.RDB.HMSet(config.Ctx, key, drugMap).Err()   // 添加/修改商品
```

### CartExist //购物车是否存在

```go
key := fmt.Sprintf("cart:%d:%d", UserId, DrugId)   // key格式
config.RDB.Exists(config.Ctx, key).Val() > 0       // 返回true/false
```

### CartUpdate //购物车数量更新

```go
key := fmt.Sprintf("cart:%d:%d", UserId, DrugId)          // key格式
config.RDB.HIncrBy(config.Ctx, key, "quantity", 1).Err()  // 数量+1
config.RDB.HIncrBy(config.Ctx, key, "quantity", -1).Err() // 数量-1
```

### CartList //购物车列表

```go
key := fmt.Sprintf("cart:%d:*", UserId)        // 匹配该用户所有商品key
config.RDB.Keys(config.Ctx, key).Val()         // 返回 []string
```

### CartDelete //购物车删除单个

```go
key := fmt.Sprintf("cart:%d:%d", UserId, DrugId)  // key格式
config.RDB.Del(config.Ctx, key).Err()             // 删除商品
```

### CartClear //购物车清空

```go
key := fmt.Sprintf("cart:%d:*", UserId)             // 匹配该用户所有商品key
keys := config.RDB.Keys(config.Ctx, key).Val()      // 获取所有key
for _, k := range keys {
	config.RDB.Del(config.Ctx, k).Err()              // 逐个删除
}
```

---

## PayNotify //支付异步回调

### Notify

```go
func Notify(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		fmt.Println("参数获取失败")
		c.String(200, "参数解析失败")
	}
	m := make(map[string]string)
	for s, strings := range c.Request.PostForm {
		m[s] = strings[0]
	}
	fmt.Println(m)
	if m["trade_status"] != "TRADE_SUCCESS" {
		c.String(200, "交易失败")
		return
	}
	orderSn := m["out_trade_no"]
	if orderSn == "" {
		c.String(200, "订单号不存在")
		return
	}
	var order model.Order

	if err := order.FindOrderByOrderSn(config.DB, orderSn); err != nil {
		c.String(200, "订单不存在")
		return
	}
	tx := config.DB.Begin()
	order.OrderType = 2
	order.PayType = 2

	if err := order.OrderSave(tx); err != nil {
		tx.Rollback()
		c.String(200, "订单状态更新失败")
		return
	}
	var drug model.DrugAdd

	if err := drug.FindDrugById(tx, int64(order.DrugId)); err != nil {
		tx.Rollback()
		c.String(200, "药品信息不存在")
		return
	}
	drug.Stock -= order.Number

	if err := drug.Save(tx); err != nil {
		tx.Rollback()
		c.String(200, "修改失败")
		return
	}
	tx.Commit()
	c.String(200, "交易成功")
}
```

---

## Schedule //排班管理

### ScheduleAdd

```go
func (s *DoctorService) ScheduleAdd(ctx context.Context, req *pb.ScheduleAddRequest) (resp *pb.ScheduleAddResponse, err error) {
	var doctor model.Doctor
	err = doctor.FindDoctorById(config.DB, req.DoctorId)
	if err != nil {
		return nil, errors.New("医生不存在")
	}

	var department model.Department
	err = department.FindDepartmentById(config.DB, req.DepartmentId)
	if err != nil {
		return nil, errors.New("科室不存在")
	}

	parseDate, _ := time.Parse("2006-01-02", req.ScheduleDate)

	nowData := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
	if parseDate.Before(nowData) {
		return nil, errors.New("排班不能为过去的时间")
	}

	var schedule model.Schedule
	err = schedule.FindScheduleById(config.DB, req.DoctorId, req.DepartmentId, req.ScheduleDate, req.ScheduleTime)
	if err == nil {
		return nil, errors.New("不能重复排班")
	}

	schedule = model.Schedule{
		DoctorId:     req.DoctorId,
		DepartmentId: req.DepartmentId,
		ScheduleDate: &parseDate,
		ScheduleTime: req.ScheduleTime,
		TotalNum:     int64(req.TotalNum),
		UserNum:      0,
		Status:       0,
	}

	err = schedule.Add(config.DB)
	if err != nil {
		return nil, errors.New("排班失败")
	}

	return &pb.ScheduleAddResponse{Id: int64(schedule.ID)}, nil
}
```

---

## Appointment //预约管理

### AppointmentAdd

```go
func (s *UserService) AppointmentAdd(ctx context.Context, req *pb.AppointmentAddRequest) (resp *pb.AppointmentAddResponse, err error) {
	// 1.时间格式
	parseDate, _ := time.Parse("2006-01-02", req.ScheduleDate)
	nowDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
	if parseDate.Before(nowDate) {
		return nil, errors.New("不能预约过去的日期")
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 2.不能重复预约
	var appointment model.Appointment
	count := appointment.FindAppointment(tx, req.UserId, req.DoctorId, req.ScheduleId, req.ScheduleDate, req.ScheduleTime)
	if count > 0 {
		tx.Rollback()
		return nil, errors.New("不能重复预约")
	}

	// 3.查询医生状态
	var doctor model.Doctor
	err = doctor.FindDoctorById(tx, req.DoctorId)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("医生不存在")
	}

	if doctor.Status != 1 {
		tx.Rollback()
		return nil, errors.New("医生未出诊")
	}

	var schedule model.Schedule
	err = schedule.FindSchedule(tx, req.ScheduleId)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("排班信息不存在")
	}

	if schedule.UserNum >= schedule.TotalNum {
		tx.Rollback()
		return nil, errors.New("号源不足")
	}

	appointmentNo := pkg.AppointmentSn()
	price := doctor.Service

	expireTime := time.Now().Add(30 * time.Minute)

	// 1.添加预约
	appointment = model.Appointment{
		Model:           gorm.Model{},
		UserId:          req.UserId,
		DoctorId:        req.DoctorId,
		ScheduleId:      req.ScheduleId,
		AppointmentDate: &parseDate,
		AppointmentTime: req.ScheduleTime,
		AppointmentNo:   appointmentNo,
		Status:          1,
		Total:           price,
		PayTypes:        0,
		ExpireTime:      &expireTime,
	}

	if err = appointment.Add(tx); err != nil {
		tx.Rollback()
		return nil, errors.New("预约记录添加失败")
	}

	// 2.扣减号源
	schedule.UserNum++

	if err = schedule.Save(tx); err != nil {
		tx.Rollback()
		return nil, errors.New("号源更新失败")
	}

	err = tx.Commit().Error
	if err != nil {
		return nil, errors.New("事务提交失败")
	}

	PayUrl := pkg.Pay(appointmentNo, price)

	return &pb.AppointmentAddResponse{
		AppointmentNo: appointmentNo,
		Total:         float32(price),
		PayUrl:       PayUrl,
	}, nil
}
```

### RestoreNum //定时任务-号源恢复

```go
func RestoreNum() {
	fmt.Println("我是计划任务")

	var appointment []model.Appointment
	config.DB.Where("expire_time < ? AND status = ?", time.Now(), 1).Find(&appointment)

	if len(appointment) > 0 {
		for _, app := range appointment {
			app.Status = 3
			err := app.Save(config.DB)
			if err != nil {
				return
			}

			var schedule model.Schedule
			err = config.DB.Where("id = ?", app.ScheduleId).First(&schedule).Error
			if err != nil {
				return
			}

			schedule.UserNum -= 1

			err = schedule.Save(config.DB)
			if err != nil {
				return
			}

		}
		fmt.Println("号源释放成功")
	}
}
```

---

## License

MIT License