package zorm

//IEntityStruct structe实体类的接口,所有的struct实体类都要实现这个接口
type IEntityStruct interface {
	//获取表名称
	GetTableName() string
	//获取数据库表的主键字段名称.因为要兼容Map,只能是数据库的字段名称.
	GetPKColumnName() string
	//GetPkSequence 主键序列,需要兼容多种数据库的序列,使用map,key是DBType,value是序列的值,例如oracle的TESTSEQ.NEXTVAL,如果有值,优先级最高
	//如果key对应的value是 "",则代表是触发器触发的序列,兼容自增关键字,例如 ["oracle"]""
	GetPkSequence() map[string]string
}

//IEntityMap 使用Map保存数据,用于不方便使用struct的场景,如果主键是自增或者序列,不要entityMap.Set主键的值
type IEntityMap interface {
	//获取表名称
	GetTableName() string
	//获取数据库表的主键字段名称.因为要兼容Map,只能是数据库的字段名称.
	GetPKColumnName() string
	//GetPkSequence 主键序列,需要兼容多种数据库的序列,使用map,key是DBType,value是序列的值,例如oracle的TESTSEQ.NEXTVAL,如果有值,优先级最高
	//如果key对应的value是 "",则代表是触发器触发的序列,兼容自增关键字,例如 ["oracle"]""
	GetPkSequence() map[string]string
	//针对Map类型,记录数据库字段
	GetDBFieldMap() map[string]interface{}
	//设置数据库字段的值
	Set(key string, value interface{}) map[string]interface{}
}

//EntityStruct IBaseEntity 的基础实现,所有的实体类都匿名注入.这样就类似实现继承了,如果接口增加方法,调整这个默认实现即可
type EntityStruct struct {
}

//默认数据库的主键列名
const defaultPkName = "id"

//获取表名称
/*
func (entity *EntityStruct) GetTableName() string {
	return ""
}
*/

//GetPKColumnName 获取数据库表的主键字段名称.因为要兼容Map,只能是数据库的字段名称.
func (entity *EntityStruct) GetPKColumnName() string {
	return defaultPkName
}

//var defaultPkSequence = make(map[string]string, 0)

//GetPkSequence 主键序列,需要兼容多种数据库的序列,使用map,key是DBType,value是序列的值,例如oracle的TESTSEQ.NEXTVAL,如果有值,优先级最高
//如果key对应的value是 "",则代表是触发器触发的序列,兼容自增关键字,例如 ["oracle"]""
func (entity *EntityStruct) GetPkSequence() map[string]string {
	return nil
}

//-------------------------------------------------------------------------//

//EntityMap IEntityMap的基础实现,可以直接使用或者匿名注入
type EntityMap struct {
	//表名
	tableName string
	//主键列名
	PkColumnName string
	//主键序列,需要兼容多种数据库的序列,使用map,key是DBType,value是序列的值,例如oracle的TESTSEQ.NEXTVAL,如果有值,优先级最高
	PkSequence map[string]string
	//数据库字段,不暴露外部
	dbFieldMap map[string]interface{}
}

//NewEntityMap 初始化Map,必须传入表名称
func NewEntityMap(tbName string) *EntityMap {
	entityMap := EntityMap{}
	entityMap.dbFieldMap = map[string]interface{}{}
	entityMap.tableName = tbName
	entityMap.PkColumnName = defaultPkName
	return &entityMap
}

//GetTableName 获取表名称
func (entity *EntityMap) GetTableName() string {
	return entity.tableName
}

//GetPKColumnName 获取数据库表的主键字段名称.因为要兼容Map,只能是数据库的字段名称.
func (entity *EntityMap) GetPKColumnName() string {
	return entity.PkColumnName
}

//GetPkSequence 主键序列,需要兼容多种数据库的序列,使用map,key是DBType,value是序列的值,例如oracle的TESTSEQ.NEXTVAL,如果有值,优先级最高
//如果key对应的value是 "",则代表是触发器触发的序列,兼容自增关键字,例如 ["oracle"]""
func (entity *EntityMap) GetPkSequence() map[string]string {
	return entity.PkSequence
}

//GetDBFieldMap 针对Map类型,记录数据库字段
func (entity *EntityMap) GetDBFieldMap() map[string]interface{} {
	return entity.dbFieldMap
}

//Set 设置数据库字段
func (entity *EntityMap) Set(key string, value interface{}) map[string]interface{} {
	entity.dbFieldMap[key] = value
	return entity.dbFieldMap
}
