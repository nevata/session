package session

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// var once sync.Once

type ManagerOption struct {
	mMaxLifeTime int64 //单位：秒
	mOnUpdate    func(sid string, t time.Time)
	mOnSave      func(sid string, sdata []byte)
	mOnTimeout   func(sid string)
}

type Manager struct {
	mLock     sync.RWMutex
	mSessions map[string]*Session
	mOption   ManagerOption
}

type ModManagerOption func(opt *ManagerOption)

func NewManager(modOption ModManagerOption) *Manager {
	once.Do(func() {
		gob.Register(&time.Time{})
		rand.Seed(time.Now().UnixNano())
	})

	option := ManagerOption{
		mMaxLifeTime: 3600, //默认1小时
	}

	modOption(&option)

	mgr := Manager{
		mSessions: make(map[string]*Session),
		mOption:   option,
	}

	go mgr.gc()

	return &mgr
}

//gc 删除超时的session
func (mgr *Manager) gc() {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	for sid, sess := range mgr.mSessions {
		timeout := sess.mLastTimeAccessed.Unix() + mgr.mOption.mMaxLifeTime
		if timeout < time.Now().Unix() {
			if mgr.mOption.mOnTimeout != nil {
				mgr.mOption.mOnTimeout(sid)
			}
			delete(mgr.mSessions, sid)
		}
	}

	time.AfterFunc(1*time.Minute, func() {
		mgr.gc()
	})
}

//generateSessionID 生成sessionID
func (mgr *Manager) generateSessionID() string {
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

//EndSessionBy 结束session
func (mgr *Manager) EndSession(sessionID string) {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	delete(mgr.mSessions, sessionID)
}

//StartSession 创建session
func (mgr *Manager) StartSession(w http.ResponseWriter, r *http.Request, userID string) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	sid := url.QueryEscape(mgr.generateSessionID())
	session := &Session{
		mSessionID:        sid,
		mUserID:           userID,
		mLastTimeAccessed: time.Now(),
		mValue:            make(map[string]interface{}),
		mOnSaveGob:        mgr.mOption.mOnSave,
	}
	mgr.mSessions[sid] = session

	return session
}

//GetSession 获取session，并更新最后访问时间
func (mgr *Manager) GetSession(w http.ResponseWriter, r *http.Request) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	auth := r.Header.Get("Authorization")
	if len(auth) <= 9 || strings.ToUpper(auth[0:10]) != "DSSESSION " {
		log.Printf("[session]Authorization(%s)不正确\n", auth)
		return nil
	}

	decodeBytes, err := base64.StdEncoding.DecodeString(auth[10:])
	if err != nil {
		log.Printf("[session]Authorization(%s)解码失败\n", auth)
		return nil
	}

	sid := string(decodeBytes)
	sess, ok := mgr.mSessions[sid]
	if ok {
		sess.mLastTimeAccessed = time.Now()
		if mgr.mOption.mOnUpdate != nil {
			mgr.mOption.mOnUpdate(sess.mSessionID, sess.mLastTimeAccessed)
		}
	}

	return sess
}

func (mgr *Manager) AddSession(sid string, sdata []byte) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	sess := Session{
		mSessionID:        sid,
		mLastTimeAccessed: time.Now(),
		mValue:            make(map[string]interface{}),
		mOnSaveGob:        mgr.mOption.mOnSave,
	}

	if len(sdata) > 0 {
		err := gob.NewDecoder(bytes.NewReader(sdata)).Decode(&sess.mValue)
		if err != nil {
			log.Println("[Session] new session gob error:", err)
		}
	}

	mgr.mSessions[sid] = &sess

	return &sess
}
