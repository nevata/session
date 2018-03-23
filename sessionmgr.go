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

func (mgr *sessionmgr) EraseSession(userID interface{}, isAdmin bool) {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	for sid, session := range mgr.mSessions {
		if uid, ok := session.mValue["userid"]; ok && (uid == userID) {
			admin, ok := session.mValue["admin"]
			if ok && admin.(bool) && isAdmin {
				delete(mgr.mSessions, sid)
				return
			}

			if (!ok || !admin.(bool)) && !isAdmin {
				delete(mgr.mSessions, sid)
				return
			}
		}
	}
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

	return session
}
