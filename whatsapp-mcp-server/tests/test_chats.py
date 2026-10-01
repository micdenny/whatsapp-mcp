import sqlite3
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
import whatsapp  # noqa: E402

GROUP = "120363000000000001@g.us"
DIRECT = "393330000001@s.whatsapp.net"
QUIET = "120363000000000002@g.us"
LAST = "2026-10-01 08:28:42+02:00"


@pytest.fixture
def store(tmp_path, monkeypatch):
    """A messages.db shaped like the bridge's, and no contact store beside it."""
    db = tmp_path / "messages.db"
    conn = sqlite3.connect(db)
    conn.executescript("""
        CREATE TABLE chats (jid TEXT PRIMARY KEY, name TEXT, last_message_time TIMESTAMP);
        CREATE TABLE messages (
            id TEXT, chat_jid TEXT, sender TEXT, content TEXT, timestamp TIMESTAMP,
            is_from_me BOOLEAN, media_type TEXT, filename TEXT,
            PRIMARY KEY (id, chat_jid)
        );
    """)
    conn.executemany("INSERT INTO chats VALUES (?, ?, ?)", [
        (GROUP, "Team parents", LAST),
        (DIRECT, "Coach", "2026-09-30 15:00:00+02:00"),
        (QUIET, "Old group", "2026-09-01 10:00:00+02:00"),
    ])
    conn.executemany("INSERT INTO messages VALUES (?, ?, ?, ?, ?, ?, ?, ?)", [
        # A document and its caption share the chat's last timestamp.
        ("A1", GROUP, "393330000001", "", LAST, 0, "document", "plan.pdf"),
        ("A2", GROUP, "393330000001", "Caption of the plan", LAST, 0, None, None),
        ("B1", DIRECT, "393330000001", "See you", "2026-09-30 15:00:00+02:00", 0, None, None),
        ("C1", QUIET, "393330000002", "Bye", "2026-09-01 10:00:00+02:00", 0, None, None),
    ])
    conn.commit()
    conn.close()
    monkeypatch.setattr(whatsapp, "MESSAGES_DB_PATH", str(db))
    monkeypatch.setattr(whatsapp, "CONTACTS_DB_PATH", str(tmp_path / "missing.db"))
    return db


def test_list_chats_without_last_message_returns_chats(store):
    chats = whatsapp.list_chats(include_last_message=False)

    assert [c.jid for c in chats] == [GROUP, DIRECT, QUIET]
    assert all(c.last_message is None for c in chats)


def test_list_chats_returns_each_chat_once(store):
    chats = whatsapp.list_chats()

    assert [c.jid for c in chats] == [GROUP, DIRECT, QUIET]


def test_list_chats_takes_the_latest_stored_of_tied_messages(store):
    group = whatsapp.list_chats(query="Team")[0]

    assert group.last_message == "Caption of the plan"


def test_list_chats_paginates_by_chat_not_by_message(store):
    first = whatsapp.list_chats(limit=1, page=0)
    second = whatsapp.list_chats(limit=1, page=1)

    assert [c.jid for c in first + second] == [GROUP, DIRECT]


def test_list_chats_sorts_by_name(store):
    chats = whatsapp.list_chats(sort_by="name", include_last_message=False)

    assert [c.name for c in chats] == ["Coach", "Old group", "Team parents"]


def test_get_chat_without_last_message(store):
    chat = whatsapp.get_chat(GROUP, include_last_message=False)

    assert chat is not None and chat.name == "Team parents"
    assert chat.last_message is None


def test_get_chat_with_last_message(store):
    assert whatsapp.get_chat(GROUP).last_message == "Caption of the plan"


def test_get_contact_chats_lists_each_chat_once(store):
    chats = whatsapp.get_contact_chats("393330000001")

    assert [c.jid for c in chats] == [GROUP, DIRECT]


def test_get_direct_chat_by_contact(store):
    chat = whatsapp.get_direct_chat_by_contact("393330000001")

    assert chat.jid == DIRECT and chat.last_message == "See you"
