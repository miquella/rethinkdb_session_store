package rethinkdb_session_store

import (
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
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.Codecs...)
	if err != nil {
		return err
	}

	value := map[string]interface{}{"encoded": encoded}
	if session.ID != "" {
		_, err := s.term.Get(session.ID).Update(value).Run(s.rethinkdbSession)
		if err != nil {
			return err
		}
	} else {
		write, err := s.term.Insert(value).RunWrite(s.rethinkdbSession)
		if err != nil {
			return err
		}
		session.ID = write.GeneratedKeys[0]
	}
	return nil
}

func (s *RethinkDBStore) load(session *sessions.Session) error {
	value := map[string]interface{}{}
	cursor, err := s.term.Get(session.ID).Run(s.rethinkdbSession)
	if err != nil {
		return err
	}
	err = cursor.One(&value)
	if err != nil {
		return err
	}

	if err = securecookie.DecodeMulti(session.Name(), value["encoded"].(string), &session.Values, s.Codecs...); err != nil {
		return err
	}
	return nil
}
