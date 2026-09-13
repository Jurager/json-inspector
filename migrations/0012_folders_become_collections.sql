-- Папка — это коллекция с родителем, и отдельной сущности у неё больше нет.
--
-- Схема 0003 завела две таблицы и держала в collection_nodes и папку, и запрос, чтобы перенос узла
-- был обновлением parent_id, а не переездом между таблицами. Но папка от коллекции отличается
-- ровно одним: у коллекции нет родителя. Из-за этого дерево умело только то, что умело — уровень
-- коллекций был один, верхний, и положить коллекцию в коллекцию было нечем.
--
-- Здесь `collections` получает parent_id, а всё, что было папкой, переезжает в неё **со своим же
-- id**. Id папки уже держит на себе её уровень: scripts_json и auth_json лежат в самой строке и
-- адресуются по id через UNION по collections, collection_nodes и drafts (script_store.go), а
-- история прогонов ссылается на папку именем (node_id у collection_runs — обычный TEXT без ключа,
-- 0008). Свежий id потерял бы и скрипты, и авторизацию, и историю, а строка папки в
-- collection_nodes осталась бы рядом — и Scripts() читал бы её, тогда как SaveScripts() писал бы в
-- collections: уровень молча раздвоился бы.
--
-- collection_nodes пересобирается, а не чистится по колонкам: kind участвует в CHECK, parent_id —
-- в индексе, и DROP COLUMN на обоих отказывает. Идёт штатный рецепт (rename → create → insert →
-- drop) без выключения foreign_keys — он приходит из DSN и внутри транзакции всё равно не
-- переключается, а ссылок на collection_nodes извне нет: единственная её собственная.

-- 1. Коллекция может лежать в коллекции. Пусто = верхний уровень: у колонки с REFERENCES не бывает
--    значения по умолчанию, а NULL и есть «родителя нет» — как parent_id узлов читался через ifnull.
ALTER TABLE collections ADD COLUMN parent_id TEXT REFERENCES collections(id) ON DELETE CASCADE;

-- 2. Папки становятся коллекциями — сначала без родителей, чтобы ни одна ссылка не ждала строки,
--    которой ещё нет. Порядок строк в одном INSERT ... SELECT схемой не обещан, и полагаться на
--    «родитель вставлен раньше ребёнка» здесь не за что.
INSERT INTO collections (id, name, description, position, auth_json, scripts_json,
                         created_at, updated_at)
SELECT id, name, coalesce(description, ''), position, auth_json, scripts_json,
       created_at, updated_at
  FROM collection_nodes WHERE kind = 'folder';

-- 3. Теперь родителя можно проставить всем: строки уже на месте. Верхний уровень — это коллекция,
--    внутри которой папка лежала; всё остальное — папка, внутри которой лежала она сама.
UPDATE collections
   SET parent_id = (SELECT CASE WHEN coalesce(n.parent_id, '') = '' THEN n.collection_id
                               ELSE n.parent_id END
                      FROM collection_nodes n WHERE n.id = collections.id)
 WHERE EXISTS (SELECT 1 FROM collection_nodes n
                WHERE n.id = collections.id AND n.kind = 'folder');

-- 4. Прогон папки становится прогоном коллекции. Строка прогона папки лежит как (collection_id
--    родителя, node_id = папка); после переезда папка — это коллекция, и её прогон есть прогон всей
--    этой коллекции, то есть (id папки, пусто). Запросов это не касается: node_id запроса папкой не
--    бывает ни при каком раскладе. Шаг идёт после второго: collection_id ссылается на collections,
--    и записать в него id папки можно только тогда, когда папка там уже лежит.
UPDATE collection_runs SET collection_id = node_id, node_id = ''
 WHERE node_id IN (SELECT id FROM collection_nodes WHERE kind = 'folder');

-- 5. Содержимое папки теперь принадлежит ей как коллекции. Позиции переносить не нужно: соседи
--    папки нумеровались в том же пространстве, в каком нумеруются дети коллекции.
UPDATE collection_nodes SET collection_id = parent_id, parent_id = NULL
 WHERE coalesce(parent_id, '') <> '';

-- 6. Запрос — это строка коллекции, и ничего больше. Ни вида, ни своего родителя у него нет.
ALTER TABLE collection_nodes RENAME TO collection_nodes_legacy;

CREATE TABLE collection_nodes (
  id TEXT PRIMARY KEY,
  collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL,
  description TEXT,
  auth_json TEXT,
  scripts_json TEXT,
  method TEXT,
  url TEXT,
  params_json TEXT,
  headers_json TEXT,
  body TEXT,
  body_kind TEXT,
  cookies_json TEXT,
  form_json TEXT,
  body_file TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

INSERT INTO collection_nodes
SELECT id, collection_id, name, position, description, auth_json, scripts_json, method, url,
       params_json, headers_json, body, body_kind, cookies_json, form_json, body_file,
       created_at, updated_at
  FROM collection_nodes_legacy
 WHERE kind = 'request';

-- Индекс уходит вместе с legacy-таблицей, поэтому новый заводится после неё — иначе имя занято.
DROP TABLE collection_nodes_legacy;

CREATE INDEX collection_nodes_tree ON collection_nodes(collection_id, position);
