package session

import (
	"time"
)

//Session session对象
type Session struct {
	mSessionID        string
	mUserID           string
	mLastTimeAccessed time.Time
	mValue            map[string]interface{}
}

//HasData 查找数据
func (sess *Session) HasData(key string) bool {
	_, ok := sess.mValue[key]
	return ok
}

//GetData 获取数据
func (sess *Session) GetData(key string) interface{} {
	return sess.mValue[key]
}

//PutData 存储数据
func (sess *Session) PutData(key string, value interface{}) {
	//log.Println("put data: ", this, key, value)
	sess.mValue[key] = value
}

//RemoveData 移除数据
func (sess *Session) RemoveData(key string) {
	delete(sess.mValue, key)
}

//SessID sid
func (sess *Session) SessID() string {
	return sess.mSessionID
}

//UserID 用户ID
func (sess *Session) UserID() string {
	return sess.mUserID
}
