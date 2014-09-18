rethinkdb_session_store
=======================

RethinkDB session support for Gorilla Web Toolkit.

Dependencies
------------

Gorilla Web Toolkit:

    go get github.com/gorilla/securecookie
    go get github.com/gorilla/sessions

Dan Cannon's rethinkdb driver:

    go get github.com/dancannon/gorethink

Usage
-----

    import (
      "github.com/dancannon/gorethink"
      "github.com/miquella/rethinkdb_session_store"
    )

    rsession, _ := rdb.Connect(rdb.ConnectOpts{Address:  "localhost:28015"})
    store := rethinkdb_session_store.NewRethinkDBStore(rsession, "web", "sessions", []byte("secret-key"))

    ...

    func handler(w http.ResponseWriter, r *http.Request) {
      session, _ := store.Get(r, "session")
      session.Values["foo"] = "bar"
      session.Save(r, w)
    }
