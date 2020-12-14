package zorm

import (
	"bytes"
	"encoding/gob"
	"errors"
	"go/ast"
	"reflect"
	"sync"
)

//allowBaseTypeMap 允许基础类型查询,用于查询单个基础类型字段,例如 select id from t_user 查询返回的是字符串类型
var allowBaseTypeMap = map[reflect.Kind]bool{
	reflect.String: true,

	reflect.Int:   true,
	reflect.Int8:  true,
	reflect.Int16: true,
	reflect.Int32: true,
	reflect.Int64: true,

	reflect.Uint:   true,
	reflect.Uint8:  true,
	reflect.Uint16: true,
	reflect.Uint32: true,
	reflect.Uint64: true,

	reflect.Float32: true,
	reflect.Float64: true,
}

const (
	//tag标签的名称
	tagColumnName = "column"

	//输出字段 缓存的前缀
	exportPrefix = "_exportStructFields_"
	//私有字段 缓存的前缀
	privatePrefix = "_privateStructFields_"
	//数据库列名 缓存的前缀
	dbColumnNamePrefix = "_dbColumnName_"

	//field对应的column的tag值 缓存的前缀
	structFieldTagPrefix = "_structFieldTag_"
	//数据库主键  缓存的前缀
	dbPKNamePrefix = "_dbPKName_"
)

// 用于缓存反射的信息,sync.Map内部处理了并发锁
//var cacheStructFieldInfoMap *sync.Map = &sync.Map{}
var cacheStructFieldInfoMap = make(map[string]map[string]reflect.StructField)

//用于缓存field对应的column的tag值
//var cacheStructFieldTagInfoMap = make(map[string]map[string]string)

//获取StructField的信息.只对struct或者*struct判断,如果是指针,返回指针下实际的struct类型.
//第一个返回值是可以输出的字段(首字母大写),第二个是不能输出的字段(首字母小写)
func structFieldInfo(typeOf reflect.Type) error {

	if typeOf == nil {
		return errors.New("数据为空")
	}

	entityName := typeOf.String()

	//缓存的key
	exportCacheKey := exportPrefix + entityName
	privateCacheKey := privatePrefix + entityName
	dbColumnCacheKey := dbColumnNamePrefix + entityName
	//structFieldTagCacheKey := structFieldTagPrefix + entityName
	//dbPKNameCacheKey := dbPKNamePrefix + entityName
	//缓存的数据库主键值
	//_, exportOk := cacheStructFieldInfoMap.Load(exportCacheKey)
	_, exportOk := cacheStructFieldInfoMap[exportCacheKey]
	//如果存在值,认为缓存中有所有的信息,不再处理
	if exportOk {
		return nil
	}
	//获取字段长度
	fieldNum := typeOf.NumField()
	//如果没有字段
	if fieldNum < 1 {
		return errors.New("entity没有属性")
	}

	// 声明所有字段的载体
	var allFieldMap *sync.Map = &sync.Map{}
	anonymous := make([]reflect.StructField, 0)

	//遍历所有字段,记录匿名属性
	for i := 0; i < fieldNum; i++ {
		field := typeOf.Field(i)
		if _, ok := allFieldMap.Load(field.Name); !ok {
			allFieldMap.Store(field.Name, field)
		}
		if field.Anonymous { //如果是匿名的
			anonymous = append(anonymous, field)
		}
	}
	//调用匿名struct的递归方法
	recursiveAnonymousStruct(allFieldMap, anonymous)

	//缓存的数据
	exportStructFieldMap := make(map[string]reflect.StructField)
	privateStructFieldMap := make(map[string]reflect.StructField)
	dbColumnFieldMap := make(map[string]reflect.StructField)
	structFieldTagMap := make(map[string]string)

	//遍历sync.Map,要求输入一个func作为参数
	//这个函数的入参、出参的类型都已经固定，不能修改
	//可以在函数体内编写自己的代码,调用map中的k,v
	f := func(k, v interface{}) bool {
		// fmt.Println(k, ":", v)
		field := v.(reflect.StructField)
		fieldName := field.Name
		if ast.IsExported(fieldName) { //如果是可以输出的
			exportStructFieldMap[fieldName] = field
			//如果是数据库字段
			tagColumnValue := field.Tag.Get(tagColumnName)
			if len(tagColumnValue) > 0 {
				dbColumnFieldMap[tagColumnValue] = field
				structFieldTagMap[fieldName] = tagColumnValue
			}

		} else { //私有属性
			privateStructFieldMap[fieldName] = field
		}

		return true
	}
	allFieldMap.Range(f)

	//加入缓存
	//cacheStructFieldInfoMap.Store(exportCacheKey, exportStructFieldMap)
	//cacheStructFieldInfoMap.Store(privateCacheKey, privateStructFieldMap)
	//cacheStructFieldInfoMap.Store(dbColumnCacheKey, dbColumnFieldMap)

	cacheStructFieldInfoMap[exportCacheKey] = exportStructFieldMap
	cacheStructFieldInfoMap[privateCacheKey] = privateStructFieldMap
	cacheStructFieldInfoMap[dbColumnCacheKey] = dbColumnFieldMap
	//cacheStructFieldTagInfoMap[structFieldTagCacheKey] = structFieldTagMap
	return nil
}

//递归调用struct的匿名属性,就近覆盖属性.
func recursiveAnonymousStruct(allFieldMap *sync.Map, anonymous []reflect.StructField) {

	for i := 0; i < len(anonymous); i++ {
		field := anonymous[i]
		typeOf := field.Type

		if typeOf.Kind() == reflect.Ptr {
			//获取指针下的Struct类型
			typeOf = typeOf.Elem()
		}

		//只处理Struct类型
		if typeOf.Kind() != reflect.Struct {
			continue
		}

		//获取字段长度
		fieldNum := typeOf.NumField()
		//如果没有字段
		if fieldNum < 1 {
			continue
		}

		// 匿名struct里自身又有匿名struct
		anonymousField := make([]reflect.StructField, 0)

		//遍历所有字段
		for i := 0; i < fieldNum; i++ {
			field := typeOf.Field(i)
			if _, ok := allFieldMap.Load(field.Name); ok { //如果存在属性名
				continue
			} else { //不存在属性名,加入到allFieldMap
				allFieldMap.Store(field.Name, field)
			}

			if field.Anonymous { //匿名struct里自身又有匿名struct
				anonymousField = append(anonymousField, field)
			}
		}

		//递归调用匿名struct
		recursiveAnonymousStruct(allFieldMap, anonymousField)

	}

}

//根据数据库的字段名,找到struct映射的字段,并赋值
func setFieldValueByColumnName(entity interface{}, columnName string, value interface{}) error {
	//先从本地缓存中查找
	typeOf := reflect.TypeOf(entity)
	valueOf := reflect.ValueOf(entity)
	if typeOf.Kind() == reflect.Ptr { //如果是指针
		typeOf = typeOf.Elem()
		valueOf = valueOf.Elem()
	}

	dbMap, err := getDBColumnFieldMap(typeOf)
	if err != nil {
		return err
	}
	f, ok := dbMap[columnName]
	if ok { //给主键赋值
		valueOf.FieldByName(f.Name).Set(reflect.ValueOf(value))
	}
	return nil

}

//获取指定字段的值
func structFieldValue(s interface{}, fieldName string) (interface{}, error) {

	if s == nil || len(fieldName) < 1 {
		return nil, errors.New("数据为空")
	}
	//entity的s类型
	valueOf := reflect.ValueOf(s)

	kind := valueOf.Kind()
	if !(kind == reflect.Ptr || kind == reflect.Struct) {
		return nil, errors.New("必须是Struct或者*Struct类型")
	}

	if kind == reflect.Ptr {
		//获取指针下的Struct类型
		valueOf = valueOf.Elem()
		if valueOf.Kind() != reflect.Struct {
			return nil, errors.New("必须是Struct或者*Struct类型")
		}
	}

	//FieldByName方法返回的是reflect.Value类型,调用Interface()方法,返回原始类型的数据值
	value := valueOf.FieldByName(fieldName).Interface()

	return value, nil

}

//深度拷贝对象.golang没有构造函数,反射复制对象时,对象中struct类型的属性无法初始化,指针属性也会收到影响.使用深度对象拷贝
func deepCopy(dst, src interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}

func getDBColumnFieldMap(typeOf reflect.Type) (map[string]reflect.StructField, error) {
	entityName := typeOf.String()

	//dbColumnFieldMap, dbOk := cacheStructFieldInfoMap.Load(dbColumnNamePrefix + entityName)
	dbColumnFieldMap, dbOk := cacheStructFieldInfoMap[dbColumnNamePrefix+entityName]
	if !dbOk { //缓存不存在
		//获取实体类的输出字段和私有 字段
		err := structFieldInfo(typeOf)
		if err != nil {
			return nil, err
		}
		//dbColumnFieldMap, dbOk = cacheStructFieldInfoMap.Load(dbColumnNamePrefix + entityName)
		dbColumnFieldMap, dbOk = cacheStructFieldInfoMap[dbColumnNamePrefix+entityName]
	}

	/*
		dbMap, efOK := dbColumnFieldMap.(map[string]reflect.StructField)
		if !efOK {
			return nil, errors.New("缓存数据库字段异常")
		}
		return dbMap, nil
	*/
	return dbColumnFieldMap, nil
}

/*
//获取 fileName 属性 中 tag column的值
func getStructFieldTagColumnValue(typeOf reflect.Type, fieldName string) string {
	entityName := typeOf.String()
	structFieldTagMap, dbOk := cacheStructFieldTagInfoMap[structFieldTagPrefix+entityName]
	if !dbOk { //缓存不存在
		//获取实体类的输出字段和私有 字段
		err := structFieldInfo(typeOf)
		if err != nil {
			return ""
		}
		structFieldTagMap, dbOk = cacheStructFieldTagInfoMap[structFieldTagPrefix+entityName]
	}

	return structFieldTagMap[fieldName]
}
*/

//根据保存的对象,返回插入的语句,需要插入的字段,字段的值.
func columnAndValue(entity interface{}) (reflect.Type, []reflect.StructField, []interface{}, error) {
	typeOf, checkerr := checkEntityKind(entity)
	if checkerr != nil {
		return typeOf, nil, nil, checkerr
	}
	// 获取实体类的反射,指针下的struct
	valueOf := reflect.ValueOf(entity).Elem()
	//reflect.Indirect

	//先从本地缓存中查找
	//typeOf := reflect.TypeOf(entity).Elem()

	dbMap, err := getDBColumnFieldMap(typeOf)
	if err != nil {
		return typeOf, nil, nil, err
	}

	//实体类公开字段的长度
	fLen := len(dbMap)
	//接收列的数组,这里是做一个副本,避免外部更改掉原始的列信息
	columns := make([]reflect.StructField, 0, fLen)
	//接收值的数组
	values := make([]interface{}, 0, fLen)

	//遍历所有数据库属性
	for _, field := range dbMap {
		//获取字段类型的Kind
		//	fieldKind := field.Type.Kind()
		//if !allowTypeMap[fieldKind] { //不允许的类型
		//	continue
		//}

		columns = append(columns, field)
		//FieldByName方法返回的是reflect.Value类型,调用Interface()方法,返回原始类型的数据值.字段不会重名,不使用FieldByIndex()函数
		value := valueOf.FieldByName(field.Name).Interface()

		/*
			if value != nil { //如果不是nil
				timeValue, ok := value.(time.Time)
				if ok && timeValue.IsZero() { //如果是日期零时,需要设置一个初始值1970-01-01 00:00:01,兼容数据库
					value = defaultZeroTime
				}
			}
		*/

		//添加到记录值的数组
		values = append(values, value)

	}

	//缓存数据库的列

	return typeOf, columns, values, nil

}

//获取实体类主键属性名称
func entityPKFieldName(entity IEntityStruct, typeOf reflect.Type) (string, error) {

	//检查是否是指针对象
	//typeOf, checkerr := checkEntityKind(entity)
	//if checkerr != nil {
	//	return "", checkerr
	//}

	//缓存的key,TypeOf和ValueOf的String()方法,返回值不一样
	//typeOf := reflect.TypeOf(entity).Elem()

	dbMap, err := getDBColumnFieldMap(typeOf)
	if err != nil {
		return "", err
	}
	field := dbMap[entity.GetPKColumnName()]
	return field.Name, nil

}

//检查entity类型必须是*struct类型或者基础类型的指针
func checkEntityKind(entity interface{}) (reflect.Type, error) {
	if entity == nil {
		return nil, errors.New("参数不能为空,必须是*struct类型或者基础类型的指针")
	}
	typeOf := reflect.TypeOf(entity)
	if typeOf.Kind() != reflect.Ptr { //如果不是指针
		return nil, errors.New("必须是*struct类型或者基础类型的指针")
	}
	typeOf = typeOf.Elem()
	if !(typeOf.Kind() == reflect.Struct || allowBaseTypeMap[typeOf.Kind()]) { //如果不是指针
		return nil, errors.New("必须是*struct类型或者基础类型的指针")
	}
	return typeOf, nil
}

//根据数据库返回的sql.Rows,查询出列名和对应的值.废弃
/*
func columnValueMap2Struct(resultMap map[string]interface{}, typeOf reflect.Type, valueOf reflect.Value) error {


		dbMap, err := getDBColumnFieldMap(typeOf)
		if err != nil {
			return err
		}

		for column, columnValue := range resultMap {
			field, ok := dbMap[column]
			if !ok {
				continue
			}
			fieldName := field.Name
			if len(fieldName) < 1 {
				continue
			}
			//反射获取字段的值对象
			fieldValue := valueOf.FieldByName(fieldName)
			//获取值类型
			kindType := fieldValue.Kind()
			valueType := fieldValue.Type()
			if kindType == reflect.Ptr { //如果是指针类型的属性,查找指针下的类型
				kindType = fieldValue.Elem().Kind()
				valueType = fieldValue.Elem().Type()
			}
			kindTypeStr := kindType.String()
			valueTypeStr := valueType.String()
			var v interface{}
			if kindTypeStr == "string" || valueTypeStr == "string" { //兼容string的扩展类型
				v = columnValue.String()
			} else if kindTypeStr == "int" || valueTypeStr == "int" { //兼容int的扩展类型
				v = columnValue.Int()
			}
			//bug(springrain)这个地方还要添加其他类型的判断,参照ColumnValue.go文件

			fieldValue.Set(reflect.ValueOf(v))

		}

	return nil

}
*/
//根据sql查询结果,返回map.废弃
/*
func wrapMap(columns []string, values []columnValue) (map[string]columnValue, error) {
	columnValueMap := make(map[string]columnValue)
	for i, column := range columns {
		columnValueMap[column] = values[i]
	}
	return columnValueMap, nil
}
*/
