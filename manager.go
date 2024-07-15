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

	"github.com/google/uuid"
)

// var once sync.Once

// Flag 会话删除标识
type Flag uint8

const (
	//FlagClose 正常退出
	FlagClose Flag = iota + 1
	//FlagErase 踢用户
	FlagErase
	//FlagTimeout 超时
	FlagTimeout
)

// ManagerOption 会话管理参数
type ManagerOption struct {
	MaxLifeTime int64 //单位：秒
	Singled     bool  //一个用户是否可以同时登入,默认不允许
	OnCreate    func(sid string, userid interface{}) error
	OnUpdate    func(sid string, t time.Time) error
	OnSave      func(sid string, sdata []byte) error
	OnDelete    func(sid string, flag Flag) error
}

// Manager 会话管理
type Manager struct {
	mLock     sync.RWMutex
	mSessions map[string]*Session
	mOption   ManagerOption
}

// ModManagerOption 会话管理参数修改
type ModManagerOption func(opt *ManagerOption)

// NewManager 创建会话管理
func NewManager(modOption ModManagerOption) *Manager {
	once.Do(func() {
		gob.Register(&time.Time{})
		gob.Register(&uuid.UUID{})
	})

	option := ManagerOption{
		MaxLifeTime: 3600, //默认1小时
		Singled:     true,
	}

	modOption(&option)

	mgr := Manager{
		mSessions: make(map[string]*Session),
		mOption:   option,
	}

	go mgr.gc()

	return &mgr
}

// gc 删除超时的会话
func (mgr *Manager) gc() {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	for sid, sess := range mgr.mSessions {
		timeout := sess.mLastTimeAccessed.Add(time.Duration(mgr.mOption.MaxLifeTime) * time.Second)
		if timeout.Before(time.Now()) {
			if mgr.mOption.OnDelete != nil {
				if err := mgr.mOption.OnDelete(sid, FlagTimeout); err != nil {
					log.Println("[session]OnDelete failed, err: ", err)
				}
			}
			delete(mgr.mSessions, sid)
		}
	}

	time.AfterFunc(1*time.Minute, func() {
		mgr.gc()
	})
}

// generateSessionID 生成sessionID
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

// EndSession 结束会话
func (mgr *Manager) EndSession(sessionID string) {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	if err := mgr.mOption.OnDelete(sessionID, FlagClose); err != nil {
		log.Println("[session]OnDelete failed, err: ", err)
	}

	delete(mgr.mSessions, sessionID)
}

// eraseSession 结束用户会话(踢用户)
func (mgr *Manager) eraseSession(userid interface{}) {
	for k, v := range mgr.mSessions {
		if v.mUserID == userid {
			if err := mgr.mOption.OnDelete(k, FlagErase); err != nil {
				log.Println("[session]OnDelete failed, err: ", err)
			}
			delete(mgr.mSessions, k)
			return
		}
	}
}

// StartSession 创建session
func (mgr *Manager) StartSession(w http.ResponseWriter, r *http.Request, userid interface{}) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	if mgr.mOption.Singled {
		mgr.eraseSession(userid)
	}

	sid := url.QueryEscape(mgr.generateSessionID())
	session := &Session{
		mSessionID:        sid,
		mUserID:           userid,
		mLastTimeAccessed: time.Now(),
		mValue:            make(map[string]interface{}),
		mManager:          mgr,
	}
	mgr.mSessions[sid] = session

	if mgr.mOption.OnCreate != nil {
		if err := mgr.mOption.OnCreate(sid, userid); err != nil {
			log.Println("[session]OnCreate failed, err: ", err)
		}
	}

	return session
}

// GetSession 获取session，并更新最后访问时间
func (mgr *Manager) GetSession(w http.ResponseWriter, r *http.Request) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	sid := r.URL.Query().Get("sid")
	if sid == "" {
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

		sid = string(decodeBytes)
	}

	sess, ok := mgr.mSessions[sid]
	if ok {
		sess.mLastTimeAccessed = time.Now()
		if mgr.mOption.OnUpdate != nil {
			if err := mgr.mOption.OnUpdate(
				sess.mSessionID,
				sess.mLastTimeAccessed); err != nil {
				log.Println("[session]OnUpdate failed, err: ", err)
			}
		}
	}

	return sess
}

// AddSession 添加持久化的会话
func (mgr *Manager) AddSession(
	sid string,
	sdata []byte,
	userid interface{},
	lastTimeAccessed time.Time,
) *Session {
	mgr.mLock.Lock()
	defer mgr.mLock.Unlock()

	sess := Session{
		mSessionID:        sid,
		mUserID:           userid,
		mLastTimeAccessed: lastTimeAccessed,
		mValue:            make(map[string]interface{}),
		mManager:          mgr,
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
