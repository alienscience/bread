CREATE TABLE session(id, classifier BLOB, ignored BLOB, browsed NUMBER, classified NUMBER);
CREATE TABLE story(providerid, title, summary, link, comments);
CREATE TABLE read(sessionid, storyid);
CREATE UNIQUE INDEX providx on story(providerid);
CREATE UNIQUE INDEX sessidx on session(id);
CREATE UNIQUE INDEX readidx on read(sessionid, storyid);
