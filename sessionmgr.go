package session

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var ins *sessionmgr
var once sync.Once

type sessionmgr struct {
	mLock        sync.RWMutex
	mCookieName  string
	mMaxLifeTime int //单位：秒
	mSessions    map[string]*Session
}

// SessionMgr session管理器
func SessionMgr() *sessionmgr {
	once.Do(func() {
		ins = &sessionmgr{}
		ins.mCookieName = "sid"
		ins.mMaxLifeTime = 3600
		ins.mSessions = make(map[string]*Session)
		ins.gc()
		rand.Seed(time.Now().UnixNano())
	})
	return ins
}

//gc 删除超时的session
func (mgr *sessionmgr) gc() {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	for sid, session := range mgr.mSessions {
		if session.mLastTimeAccessed.Unix()+int64(mgr.mMaxLifeTime) < time.Now().Unix() {
			delete(mgr.mSessions, sid)
		}
	}

	time.AfterFunc(time.Duration(mgr.mMaxLifeTime)*time.Second, func() {
		mgr.gc()
	})
}

//generateSessionID 生成sessionID
func (mgr *sessionmgr) generateSessionID() string {
	var p1, p2, p3 int
	var sid string
	for {
		p1 = rand.Intn(1000000)
		p2 = rand.Intn(1000000)
		p3 = rand.Intn(1000000)
		sid = fmt.Sprintf("%d.%d.%d", p1, p2, p3)
		if _, found := mgr.mSessions[sid]; !found {
			break
		}
	}
	return sid
}

//EraseSession 结束session，用于踢除重复登录的用户
func (mgr *sessionmgr) EraseSession(userID string) {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()
	for sid, sess := range mgr.mSessions {
		if sess.mUserID == userID {
			delete(mgr.mSessions, sid)
			return
		}
	}
}

//EndSessionBy 结束session
func (mgr *sessionmgr) EndSession(sessionID string) {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	delete(mgr.mSessions, sessionID)
}

//StartSession 创建session
func (mgr *sessionmgr) StartSession(w http.ResponseWriter, r *http.Request, userID string) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	sid := url.QueryEscape(mgr.generateSessionID())
	session := &Session{mSessionID: sid, mUserID: userID, mLastTimeAccessed: time.Now(), mValue: make(map[string]interface{})}
	mgr.mSessions[sid] = session

	cookie := http.Cookie{Name: mgr.mCookieName, Value: sid, Path: "/", HttpOnly: true, MaxAge: mgr.mMaxLifeTime}
	http.SetCookie(w, &cookie)

	return session
}

//GetSession 获取session，并更新最后访问时间
func (mgr *sessionmgr) GetSession(w http.ResponseWriter, r *http.Request) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	var cookie, err = r.Cookie(mgr.mCookieName)
	if cookie == nil || err != nil {
		return nil
	}

	sid := cookie.Value
	session, ok := mgr.mSessions[sid]
	if !ok {
		return nil
	}

	session.mLastTimeAccessed = time.Now()
	cookie.MaxAge = mgr.mMaxLifeTime
	http.SetCookie(w, cookie)
	return session
}
