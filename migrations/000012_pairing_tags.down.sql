-- Откат миграции 000012: удаляем M2M, потом сам vocabulary.
-- FK dish_pairing_tags.tag_slug → pairing_tags.slug стоит ON DELETE CASCADE,
-- но мы всё равно дропаем явно в правильном порядке, чтобы не зависеть от порядка
-- сноса в pgloader/golang-migrate.

DROP TABLE IF EXISTS dish_pairing_tags;
DROP TABLE IF EXISTS pairing_tags;
