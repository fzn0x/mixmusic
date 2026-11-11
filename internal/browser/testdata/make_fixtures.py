"""Writes the SQLite fixtures of internal/browser's tests: Firefox's
cookie store as its current schema keeps it, with a write-ahead log; one
whose every change is still in its log; and one of an older schema, which
kept the expiry in seconds. Every value is made up.

Usage, from internal/browser:
python testdata/make_fixtures.py testdata <a scratch folder>
"""

import os
import shutil
import sqlite3
import sys

OUT, TMP = sys.argv[1], sys.argv[2]
os.makedirs(OUT, exist_ok=True)
os.makedirs(TMP, exist_ok=True)

V17 = (
    "CREATE TABLE moz_cookies ("
    "id INTEGER PRIMARY KEY, "
    "originAttributes TEXT NOT NULL DEFAULT '', "
    "name TEXT, "
    "value TEXT, "
    "host TEXT, "
    "path TEXT, "
    "expiry INTEGER, "
    "lastAccessed INTEGER, "
    "creationTime INTEGER, "
    "isSecure INTEGER, "
    "isHttpOnly INTEGER, "
    "inBrowserElement INTEGER DEFAULT 0, "
    "sameSite INTEGER DEFAULT 0, "
    "schemeMap INTEGER DEFAULT 0, "
    "isPartitionedAttributeSet INTEGER DEFAULT 0, "
    "updateTime INTEGER, "
    "CONSTRAINT moz_uniqueid UNIQUE (name, host, path, originAttributes)"
    ")"
)

V12 = (
    "CREATE TABLE moz_cookies ("
    "id INTEGER PRIMARY KEY, "
    "originAttributes TEXT NOT NULL DEFAULT '', "
    "name TEXT, "
    "value TEXT, "
    "host TEXT, "
    "path TEXT, "
    "expiry INTEGER, "
    "lastAccessed INTEGER, "
    "creationTime INTEGER, "
    "isSecure INTEGER, "
    "isHttpOnly INTEGER, "
    "inBrowserElement INTEGER DEFAULT 0, "
    "sameSite INTEGER DEFAULT 0, "
    "rawSameSite INTEGER DEFAULT 0, "
    "schemeMap INTEGER DEFAULT 0, "
    "CONSTRAINT moz_uniqueid UNIQUE (name, host, path, originAttributes)"
    ")"
)

# 2100-01-01, in seconds, and Firefox's milliseconds.
EXPIRY_S = 4102444800
EXPIRY_MS = EXPIRY_S * 1000
NOW_US = 1762819200000000


def fresh(path):
    for f in (path, path + "-wal", path + "-shm"):
        if os.path.exists(f):
            os.remove(f)
    con = sqlite3.connect(path, isolation_level=None)
    return con


def add(con, attrs, name, value, host, path="/", expiry=EXPIRY_MS, secure=1, http_only=0):
    con.execute(
        "INSERT INTO moz_cookies (originAttributes, name, value, host, path, expiry, lastAccessed,"
        " creationTime, isSecure, isHttpOnly, updateTime) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
        (attrs, name, value, host, path, expiry, NOW_US, NOW_US, secure, http_only, NOW_US),
    )


def copy(path, name):
    shutil.copyfile(path, os.path.join(OUT, name))
    if os.path.exists(path + "-wal") and os.path.getsize(path + "-wal") > 0:
        shutil.copyfile(path + "-wal", os.path.join(OUT, name + "-wal"))


# The current schema: most rows in the file, the last changes in the log.
path = os.path.join(TMP, "cookies.sqlite")
con = fresh(path)
con.execute("PRAGMA page_size = 1024")
con.execute("PRAGMA journal_mode = WAL")
con.execute("PRAGMA wal_autocheckpoint = 0")
con.execute(V17)
con.execute("CREATE INDEX moz_basedomain ON moz_cookies (host, originAttributes)")
con.execute("BEGIN")
add(con, "", "SAPISID", "old-sapisid", ".youtube.com")
add(con, "", "SID", "made-up-sid", ".youtube.com")
add(con, "", "HOSTONLY", "music-only", "music.youtube.com")
# Longer than a page holds: the rest overflows into pages of its own.
add(con, "", "LOGIN_INFO", "L" * 3000 + "end", ".youtube.com", http_only=1)
add(con, "^userContextId=1", "SAPISID", "in-a-container", ".youtube.com")
add(con, "^partitionKey=%28https%2Cexample.com%29", "NID", "partitioned", ".google.com")
add(con, "", "NID", "made-up-nid", ".google.com")
for i in range(80):
    add(con, "", "F%03d" % i, "x" * 40, ".site%d.example" % i)
con.execute("COMMIT")
con.execute("PRAGMA wal_checkpoint(TRUNCATE)")
con.execute("BEGIN")
con.execute("UPDATE moz_cookies SET value = 'new-sapisid' WHERE name = 'SAPISID' AND originAttributes = ''")
add(con, "", "__Secure-3PAPISID", "made-up-3p", ".youtube.com")
con.execute("DELETE FROM moz_cookies WHERE name = 'F000'")
con.execute("COMMIT")
copy(path, "cookies.sqlite")
con.close()

# A new database: its file holds its first page, the log all the rest.
path = os.path.join(TMP, "cookies-new.sqlite")
con = fresh(path)
con.execute("PRAGMA page_size = 1024")
con.execute("PRAGMA journal_mode = WAL")
con.execute("PRAGMA wal_autocheckpoint = 0")
con.execute(V17)
add(con, "", "SAPISID", "only-in-the-log", ".youtube.com")
copy(path, "cookies-new.sqlite")
con.close()

# An older schema, whose expiry is in seconds, with no log.
path = os.path.join(TMP, "cookies-v12.sqlite")
con = fresh(path)
con.execute("PRAGMA page_size = 4096")
con.execute(V12)
con.execute(
    "INSERT INTO moz_cookies (originAttributes, name, value, host, path, expiry, lastAccessed,"
    " creationTime, isSecure, isHttpOnly) VALUES ('', 'SAPISID', 'in-seconds', '.youtube.com', '/', ?, ?, ?, 1, 0)",
    (EXPIRY_S, NOW_US, NOW_US),
)
con.close()
copy(path, "cookies-v12.sqlite")

for f in sorted(os.listdir(OUT)):
    print(f, os.path.getsize(os.path.join(OUT, f)))
