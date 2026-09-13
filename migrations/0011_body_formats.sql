-- Пять форматов тела, ставшие настоящими: JSON, XML, Raw, Form-Data, Binary.
--
-- body_kind написан схемами 0003 и 0005 и не читался ни разу: тело всегда было текстом, а текст —
-- это и есть 'raw'. Колонка подхватывается с тем значением, которое в ней уже лежит, поэтому ни
-- одна сохранённая строка не меняет смысл и бэкфилл не нужен. То же с NULL у узлов: пустой вид
-- читается как 'raw' через domain.KindOf — так же, как отсутствие авторизации значит «взять у
-- уровня выше», а не «нет».
--
-- form_json — сетка Form-Data, где строка это либо текст, либо путь к файлу. body_file —
-- единственный путь, из которого читается тело Binary. Колонки по частям, а не один блоб: схема
-- везде устроена так (params_json, headers_json, cookies_json, auth_json, scripts_json), и единый
-- blob был бы единственным местом, где DDL скрывает своё содержимое.

ALTER TABLE drafts ADD COLUMN form_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE drafts ADD COLUMN body_file TEXT NOT NULL DEFAULT '';

ALTER TABLE collection_nodes ADD COLUMN form_json TEXT;
ALTER TABLE collection_nodes ADD COLUMN body_file TEXT;
