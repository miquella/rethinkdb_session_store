package rethinkdb_session_store

import (
	"errors"
	"net/http"

	"github.com/dancannon/gorethink"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

type RethinkDBStore struct {
	Codecs  []securecookie.Codec
	Options *sessions.Options // default configuration

	term             gorethink.Term
	rethinkdbSession *gorethink.Session
}

func NewRethinkDBStore(rethinkdbSession *gorethink.Session, db, table string, keyPairs ...[]byte) *RethinkDBStore {
	return &RethinkDBStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   "/",
			MaxAge: 86400 * 30,
		},
		term:             gorethink.Db(db).Table(table),
		rethinkdbSession: rethinkdbSession,
	}
}

func (s *RethinkDBStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *RethinkDBStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.Options
	session.Options = &opts
	session.IsNew = true
	var err error
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			err = s.load(session)
			if err == nil {
				session.IsNew = false
			}
		}
	}
	return session, err
}

func (s *RethinkDBStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if err := s.save(session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (s *RethinkDBStore) save(session *sessions.Session) error {
	values := map[string]interface{}{}
	for k, v := range session.Values {
		kstr, ok := k.(string)
		if !ok {
			return errors.New("cannot serialize non-string value key")
		}

		values[kstr] = v
	}

	json := map[string]interface{}{
		"name":   session.Name(),
		"values": values,
	}

	var write gorethink.WriteResponse
	var err error
	if session.ID != "" {
		write, err = s.term.Get(session.ID).Update(json).RunWrite(s.rethinkdbSession)
		if err == nil && write.Updated == 0 {
			json["id"] = session.ID
		}
	}

	if write.Updated == 0 {
		write, err = s.term.Insert(json).RunWrite(s.rethinkdbSession)
		if err == nil && len(write.GeneratedKeys) > 0 {
			session.ID = write.GeneratedKeys[0]
		}
	}

	return err
}

func (s *RethinkDBStore) load(session *sessions.Session) error {
	if session.ID == "" {
		return errors.New("invalid session id")
	}

	json := map[string]interface{}{}
	cursor, err := s.term.Get(session.ID).Run(s.rethinkdbSession)
	if err != nil {
		return err
	}
	err = cursor.One(&json)
	if err != nil {
		return err
	}

	values, ok := json["values"].(map[string]interface{})
	if !ok {
		return errors.New("failed to decode session")
	}

	for k, v := range values {
		session.Values[k] = v
	}

	return nil
}
