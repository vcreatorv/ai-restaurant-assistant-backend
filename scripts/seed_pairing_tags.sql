-- Одноразовый засев pairing-тегов по всем 193 блюдам.
-- Идемпотентно: ON CONFLICT DO NOTHING — повторный запуск ничего не сломает.
-- После применения нужно запустить `make embed-menu` для перестройки эмбеддингов в Qdrant.
--
-- Структура: (dish_id, tag_slug). Vocabulary см. в pairing_tags
-- (миграция 000012_pairing_tags.up.sql).

BEGIN;

INSERT INTO dish_pairing_tags (dish_id, tag_slug) VALUES
-- ── Закуски ────────────────────────────────────────────────────────────────
-- 1. Бабагануш (восточная, баклажан, тахини)
(1, 'pair_white_wine'), (1, 'pair_cocktails'), (1, 'role_aperitif'), (1, 'role_starter'), (1, 'vibe_light'),
-- 2. Баклажаны в панировке (азиатский соус, микс салатов)
(2, 'pair_white_wine'), (2, 'pair_beer_light'), (2, 'role_aperitif'), (2, 'role_starter'), (2, 'vibe_light'),
-- 3. Вителло Тоннато (ростбиф под кремовым соусом из тунца)
(3, 'pair_white_wine'), (3, 'pair_sparkling'), (3, 'role_aperitif'), (3, 'role_starter'), (3, 'occasion_date'), (3, 'vibe_light'),
-- 4. Карпаччо Денвер (тонкая говядина, пармезан, рукола)
(4, 'pair_red_wine'), (4, 'pair_white_wine'), (4, 'role_aperitif'), (4, 'role_starter'), (4, 'occasion_date'), (4, 'vibe_light'),
-- 5. Креветки в кляре (медово-горчичный)
(5, 'pair_white_wine'), (5, 'pair_sparkling'), (5, 'pair_beer_light'), (5, 'role_aperitif'), (5, 'role_starter'), (5, 'vibe_light'),
-- 6. Крылья куриные (трио соусов, для компании)
(6, 'pair_beer_light'), (6, 'pair_beer_dark'), (6, 'role_aperitif'), (6, 'role_share'), (6, 'vibe_hearty'),
-- 7. Куриные стрипсы (пряный соус)
(7, 'pair_beer_light'), (7, 'pair_lemonade'), (7, 'role_aperitif'), (7, 'occasion_kids'), (7, 'vibe_hearty'),
-- 8. Мясное плато (салями, пармская, на компанию)
(8, 'pair_red_wine'), (8, 'pair_beer_dark'), (8, 'role_aperitif'), (8, 'role_share'), (8, 'occasion_celebration'), (8, 'vibe_hearty'),
-- 9. Сырное плато (4 сыра + вишнёвый)
(9, 'pair_red_wine'), (9, 'pair_white_wine'), (9, 'role_aperitif'), (9, 'role_share'), (9, 'occasion_date'), (9, 'occasion_celebration'),
-- 10. Тартар из авокадо с креветками (свежий)
(10, 'pair_white_wine'), (10, 'pair_sparkling'), (10, 'role_aperitif'), (10, 'role_starter'), (10, 'occasion_date'), (10, 'vibe_light'), (10, 'vibe_refreshing'),
-- 11. Чесночные гренки (простая закуска)
(11, 'pair_beer_light'), (11, 'pair_beer_dark'), (11, 'pair_red_wine'), (11, 'role_aperitif'), (11, 'role_side'),
-- 57. Фокачча с розмарином
(57, 'pair_white_wine'), (57, 'pair_red_wine'), (57, 'role_aperitif'), (57, 'role_starter'), (57, 'role_side'), (57, 'vibe_light'),
-- 58. Фокачча с томатами
(58, 'pair_white_wine'), (58, 'pair_red_wine'), (58, 'role_aperitif'), (58, 'role_starter'), (58, 'role_side'), (58, 'vibe_light'),
-- 59. Хлебная корзина (тёплая фокачча + масло)
(59, 'pair_white_wine'), (59, 'pair_red_wine'), (59, 'role_starter'), (59, 'role_side'), (59, 'vibe_light'),
-- 76. Тартар из говядины (каперсы, вяленые томаты)
(76, 'pair_red_wine'), (76, 'role_aperitif'), (76, 'role_starter'), (76, 'occasion_date'), (76, 'vibe_light'),
-- 173. Ролл Филадельфия (лосось, сливочный сыр)
(173, 'pair_white_wine'), (173, 'pair_sparkling'), (173, 'role_aperitif'), (173, 'role_starter'), (173, 'vibe_light'),
-- 174. Ролл Калифорния (краб, авокадо, икра)
(174, 'pair_white_wine'), (174, 'pair_sparkling'), (174, 'role_aperitif'), (174, 'role_starter'), (174, 'vibe_light'),
-- 182. Хумус с овощами и питой (вегетарианский)
(182, 'pair_white_wine'), (182, 'pair_cocktails'), (182, 'role_aperitif'), (182, 'role_starter'), (182, 'vibe_light'),
-- 189. Большое мясное плато для компании (на 4-6 чел)
(189, 'pair_red_wine'), (189, 'pair_beer_dark'), (189, 'role_aperitif'), (189, 'role_share'), (189, 'occasion_celebration'), (189, 'vibe_hearty'),
-- 190. Сырная доска для компании
(190, 'pair_red_wine'), (190, 'pair_white_wine'), (190, 'role_aperitif'), (190, 'role_share'), (190, 'occasion_celebration'),
-- 193. Закусочный сет на компанию (крылья + гренки)
(193, 'pair_beer_light'), (193, 'pair_beer_dark'), (193, 'role_aperitif'), (193, 'role_share'), (193, 'occasion_celebration'), (193, 'vibe_hearty'),

-- ── Салаты ─────────────────────────────────────────────────────────────────
-- 12. Витаминный (капуста, морковь, сельдерей) — лёгкий
(12, 'pair_white_wine'), (12, 'pair_lemonade'), (12, 'role_starter'), (12, 'occasion_kids'), (12, 'vibe_light'), (12, 'vibe_refreshing'),
-- 13. Греческий
(13, 'pair_white_wine'), (13, 'pair_sparkling'), (13, 'role_starter'), (13, 'vibe_light'), (13, 'vibe_refreshing'),
-- 14. Пастрами салат (тёплый, мраморная говядина)
(14, 'pair_red_wine'), (14, 'pair_white_wine'), (14, 'role_starter'), (14, 'role_main'), (14, 'vibe_hearty'),
-- 15. Салат Буррата (терияки, моцарелла)
(15, 'pair_white_wine'), (15, 'pair_sparkling'), (15, 'role_starter'), (15, 'occasion_date'), (15, 'vibe_light'),
-- 16. Салат Бык-тейсти (салат-бургер, фарш, чеддер)
(16, 'pair_beer_light'), (16, 'pair_red_wine'), (16, 'role_main'), (16, 'role_main_filling'), (16, 'vibe_hearty'),
-- 17. Салат зелёный (шпинат, авокадо, цукини)
(17, 'pair_white_wine'), (17, 'role_starter'), (17, 'occasion_business_lunch'), (17, 'vibe_light'), (17, 'vibe_refreshing'),
-- 18. Капрезе
(18, 'pair_white_wine'), (18, 'pair_sparkling'), (18, 'role_starter'), (18, 'vibe_light'), (18, 'vibe_refreshing'),
-- 19. Салат с говяжьим языком и ореховым соусом
(19, 'pair_red_wine'), (19, 'pair_white_wine'), (19, 'role_starter'), (19, 'role_main'), (19, 'vibe_hearty'),
-- 20. Салат с креветками и манго
(20, 'pair_white_wine'), (20, 'pair_sparkling'), (20, 'role_starter'), (20, 'occasion_date'), (20, 'vibe_light'), (20, 'vibe_refreshing'),
-- 21. Салат с печёной тыквой и фетой
(21, 'pair_white_wine'), (21, 'role_starter'), (21, 'occasion_date'), (21, 'vibe_light'), (21, 'vibe_warming'),
-- 22. Салат с ростбифом
(22, 'pair_red_wine'), (22, 'role_starter'), (22, 'role_main'), (22, 'vibe_hearty'),
-- 23. Салат томаты со страчателлой
(23, 'pair_white_wine'), (23, 'pair_sparkling'), (23, 'role_starter'), (23, 'vibe_light'), (23, 'vibe_refreshing'),
-- 24. Цезарь с креветками
(24, 'pair_white_wine'), (24, 'role_starter'), (24, 'role_main'), (24, 'occasion_date'), (24, 'vibe_light'),
-- 25. Цезарь с курицей
(25, 'pair_white_wine'), (25, 'role_starter'), (25, 'role_main'), (25, 'occasion_business_lunch'), (25, 'vibe_light'),
-- 26. Стейк-салат (стриплойн + медово-соевая)
(26, 'pair_red_wine'), (26, 'role_main'), (26, 'occasion_business_lunch'), (26, 'vibe_hearty'),
-- 27. Тёплый салат с цыплёнком (кускус)
(27, 'pair_white_wine'), (27, 'role_starter'), (27, 'role_main'), (27, 'occasion_business_lunch'), (27, 'vibe_warming'),
-- 179. Боул с киноа и авокадо (веганский)
(179, 'pair_white_wine'), (179, 'pair_lemonade'), (179, 'role_main'), (179, 'occasion_business_lunch'), (179, 'vibe_light'),

-- ── Супы ───────────────────────────────────────────────────────────────────
-- 28. Бульон куриный с лапшой
(28, 'pair_white_wine'), (28, 'role_starter'), (28, 'occasion_kids'), (28, 'vibe_warming'), (28, 'vibe_light'),
-- 29. Классический борщ (говядина, сметана)
(29, 'pair_red_wine'), (29, 'pair_beer_dark'), (29, 'role_starter'), (29, 'vibe_warming'), (29, 'vibe_hearty'),
-- 30. Крем-суп грибной (трюфельное масло)
(30, 'pair_white_wine'), (30, 'role_starter'), (30, 'occasion_date'), (30, 'vibe_warming'), (30, 'vibe_hearty'),
-- 31. Крем-суп сырный (копчёный цыплёнок)
(31, 'pair_white_wine'), (31, 'role_starter'), (31, 'vibe_warming'), (31, 'vibe_hearty'),
-- 32. Том-ям (креветки, кальмар)
(32, 'pair_white_wine'), (32, 'pair_lemonade'), (32, 'role_starter'), (32, 'role_main'), (32, 'vibe_warming'),
-- 170. Том-ям с креветками (кокосовое молоко)
(170, 'pair_white_wine'), (170, 'pair_lemonade'), (170, 'role_starter'), (170, 'role_main'), (170, 'vibe_warming'),
-- 171. Мисо-суп с тофу
(171, 'pair_white_wine'), (171, 'pair_tea'), (171, 'role_starter'), (171, 'vibe_warming'), (171, 'vibe_light'),

-- ── Пицца и паста ──────────────────────────────────────────────────────────
-- 33. Пицца Баварская (чоризо, бекон)
(33, 'pair_beer_light'), (33, 'pair_beer_dark'), (33, 'pair_red_wine'), (33, 'role_main'), (33, 'role_share'), (33, 'vibe_hearty'),
-- 34. Пицца Маргарита
(34, 'pair_white_wine'), (34, 'pair_red_wine'), (34, 'pair_beer_light'), (34, 'role_main'), (34, 'role_share'), (34, 'occasion_kids'), (34, 'vibe_light'),
-- 35. Пицца Сырная (4 сыра)
(35, 'pair_white_wine'), (35, 'pair_red_wine'), (35, 'role_main'), (35, 'role_share'), (35, 'vibe_hearty'),
-- 36. Пицца Цезарь (курица, романо)
(36, 'pair_white_wine'), (36, 'pair_beer_light'), (36, 'role_main'), (36, 'role_share'), (36, 'vibe_light'),
-- 37. Пицца Чоризо
(37, 'pair_red_wine'), (37, 'pair_beer_dark'), (37, 'role_main'), (37, 'role_share'), (37, 'vibe_hearty'),
-- 62. Паста болоньезе
(62, 'pair_red_wine'), (62, 'role_main'), (62, 'role_main_filling'), (62, 'vibe_hearty'),
-- 63. Паста с томлёной щекой
(63, 'pair_red_wine'), (63, 'role_main'), (63, 'role_main_filling'), (63, 'occasion_date'), (63, 'vibe_hearty'),
-- 176. Карбонара
(176, 'pair_white_wine'), (176, 'pair_red_wine'), (176, 'role_main'), (176, 'vibe_hearty'),
-- 177. Паста Аррабиата (острая)
(177, 'pair_red_wine'), (177, 'pair_beer_light'), (177, 'role_main'), (177, 'vibe_warming'),
-- 178. Ризотто с белыми грибами (трюфель)
(178, 'pair_white_wine'), (178, 'role_main'), (178, 'occasion_date'), (178, 'vibe_hearty'),

-- ── Бургеры ────────────────────────────────────────────────────────────────
-- 86. Бургер Jack Daniel's (томлёная говядина)
(86, 'pair_beer_dark'), (86, 'pair_red_wine'), (86, 'role_main'), (86, 'role_main_filling'), (86, 'vibe_hearty'),
-- 87. Бургер THE БЫК (Блэк Ангус, халапеньо)
(87, 'pair_beer_light'), (87, 'pair_beer_dark'), (87, 'role_main'), (87, 'role_main_filling'), (87, 'vibe_hearty'),
-- 88. Бургер Вишнёвый бизон (бекон, чеддер)
(88, 'pair_beer_dark'), (88, 'pair_red_wine'), (88, 'role_main'), (88, 'role_main_filling'), (88, 'vibe_hearty'),
-- 89. Бургер с креветками
(89, 'pair_white_wine'), (89, 'pair_beer_light'), (89, 'pair_cider'), (89, 'role_main'), (89, 'vibe_light'),
-- 90. Чизбургер (классика)
(90, 'pair_beer_light'), (90, 'pair_red_wine'), (90, 'role_main'), (90, 'role_main_filling'), (90, 'occasion_kids'), (90, 'vibe_hearty'),

-- ── Стейки ─────────────────────────────────────────────────────────────────
-- 64. Рибай Блэк Ангус
(64, 'pair_red_wine'), (64, 'role_main'), (64, 'role_main_filling'), (64, 'occasion_date'), (64, 'occasion_celebration'), (64, 'vibe_hearty'),
-- 67. Свиной томагавк
(67, 'pair_red_wine'), (67, 'pair_beer_dark'), (67, 'role_main'), (67, 'role_main_filling'), (67, 'occasion_celebration'), (67, 'vibe_hearty'),
-- 69. Стейк NY травяной
(69, 'pair_red_wine'), (69, 'role_main'), (69, 'role_main_filling'), (69, 'vibe_hearty'),
-- 70. Стейк Бавет (зерно, пашина)
(70, 'pair_red_wine'), (70, 'role_main'), (70, 'role_main_filling'), (70, 'vibe_hearty'),
-- 72. Стейк Рибай (трава)
(72, 'pair_red_wine'), (72, 'role_main'), (72, 'role_main_filling'), (72, 'occasion_date'), (72, 'vibe_hearty'),
-- 73. Стейк Стриплойн (зерно)
(73, 'pair_red_wine'), (73, 'role_main'), (73, 'role_main_filling'), (73, 'vibe_hearty'),
-- 74. Стейк Филе Миньон (самый нежный)
(74, 'pair_red_wine'), (74, 'role_main'), (74, 'occasion_date'), (74, 'occasion_celebration'), (74, 'vibe_hearty'),
-- 75. Стейк Шато Бриан (торжественный)
(75, 'pair_red_wine'), (75, 'role_main'), (75, 'occasion_date'), (75, 'occasion_celebration'), (75, 'vibe_hearty'),
-- 81. Брискет (томлёная грудинка)
(81, 'pair_red_wine'), (81, 'pair_beer_dark'), (81, 'role_main'), (81, 'role_main_filling'), (81, 'vibe_hearty'),
-- 82. Пиканья на пите
(82, 'pair_red_wine'), (82, 'pair_beer_light'), (82, 'role_main'), (82, 'vibe_hearty'),
-- 83. Ребра Кальби (томлёные, брусничный)
(83, 'pair_red_wine'), (83, 'role_main'), (83, 'role_main_filling'), (83, 'vibe_hearty'),
-- 84. Стейк Фланк
(84, 'pair_red_wine'), (84, 'role_main'), (84, 'role_main_filling'), (84, 'vibe_hearty'),
-- 85. Топ Сирлойн (постный)
(85, 'pair_red_wine'), (85, 'role_main'), (85, 'role_main_filling'), (85, 'vibe_hearty'),
-- 192. Стейк-сет на двоих (рибай + бавет + 3 соуса)
(192, 'pair_red_wine'), (192, 'role_main_filling'), (192, 'role_share'), (192, 'occasion_date'), (192, 'occasion_celebration'), (192, 'vibe_hearty'),

-- ── На гриле ───────────────────────────────────────────────────────────────
-- 38. Бифштекс 4 сыра
(38, 'pair_red_wine'), (38, 'role_main'), (38, 'role_main_filling'), (38, 'vibe_hearty'),
-- 39. Говяжьи колбаски (хоспер)
(39, 'pair_beer_dark'), (39, 'pair_red_wine'), (39, 'role_main'), (39, 'role_main_filling'), (39, 'vibe_hearty'),
-- 40. Кесадилья с говядиной
(40, 'pair_beer_light'), (40, 'pair_red_wine'), (40, 'role_main'), (40, 'role_share'), (40, 'vibe_hearty'),
-- 41. Колбаска из индейки
(41, 'pair_beer_light'), (41, 'pair_white_wine'), (41, 'role_main'), (41, 'vibe_hearty'),
-- 42. Куриные колбаски
(42, 'pair_beer_light'), (42, 'role_main'), (42, 'vibe_hearty'),
-- 43. Люля-кебаб из говядины (зира, кориандр)
(43, 'pair_red_wine'), (43, 'pair_beer_dark'), (43, 'role_main'), (43, 'role_main_filling'), (43, 'vibe_warming'), (43, 'vibe_hearty'),
-- 44. Люля-кебаб из индейки
(44, 'pair_white_wine'), (44, 'pair_red_wine'), (44, 'role_main'), (44, 'vibe_hearty'),
-- 45. Медальоны из свинины (бекон)
(45, 'pair_red_wine'), (45, 'pair_beer_dark'), (45, 'role_main'), (45, 'role_main_filling'), (45, 'vibe_hearty'),
-- 46. Свиные колбаски
(46, 'pair_beer_light'), (46, 'pair_beer_dark'), (46, 'role_main'), (46, 'role_main_filling'), (46, 'vibe_hearty'),
-- 47. Стейк индейки
(47, 'pair_white_wine'), (47, 'pair_red_wine'), (47, 'role_main'), (47, 'vibe_hearty'),
-- 48. Шашлык из курицы (фермерская)
(48, 'pair_white_wine'), (48, 'pair_red_wine'), (48, 'role_main'), (48, 'role_share'), (48, 'vibe_hearty'),
-- 49. Шашлык из свиной шеи
(49, 'pair_red_wine'), (49, 'pair_beer_dark'), (49, 'role_main'), (49, 'role_main_filling'), (49, 'role_share'), (49, 'occasion_celebration'), (49, 'vibe_hearty'),
-- 183. Шашлык из ягнёнка халяль
(183, 'pair_red_wine'), (183, 'role_main'), (183, 'role_main_filling'), (183, 'role_share'), (183, 'occasion_celebration'), (183, 'vibe_hearty'),
-- 184. Куриный шашлык в маринаде sriracha (острый)
(184, 'pair_beer_light'), (184, 'pair_red_wine'), (184, 'role_main'), (184, 'vibe_warming'),

-- ── Мясное ─────────────────────────────────────────────────────────────────
-- 50. Бефстроганов (сливочный, шампиньоны)
(50, 'pair_red_wine'), (50, 'role_main'), (50, 'role_main_filling'), (50, 'vibe_warming'), (50, 'vibe_hearty'),
-- 51. Говядина по-тайски (wok, арахис)
(51, 'pair_white_wine'), (51, 'pair_red_wine'), (51, 'role_main'), (51, 'vibe_warming'),
-- 52. Говяжьи котлеты с пюре
(52, 'pair_red_wine'), (52, 'role_main'), (52, 'role_main_filling'), (52, 'occasion_kids'), (52, 'vibe_hearty'),
-- 53. Рваная говядина на стрипсах
(53, 'pair_red_wine'), (53, 'pair_beer_dark'), (53, 'role_main'), (53, 'role_main_filling'), (53, 'vibe_hearty'),
-- 54. Свиные ребра «Барбекю»
(54, 'pair_beer_dark'), (54, 'pair_red_wine'), (54, 'role_main'), (54, 'role_main_filling'), (54, 'vibe_hearty'),
-- 55. Свиные ребра «Терияки»
(55, 'pair_white_wine'), (55, 'pair_red_wine'), (55, 'role_main'), (55, 'role_main_filling'), (55, 'vibe_hearty'),
-- 56. Свиные ребра Jack Daniel's
(56, 'pair_beer_dark'), (56, 'pair_red_wine'), (56, 'role_main'), (56, 'role_main_filling'), (56, 'vibe_hearty'),
-- 65. Рябчик в глазури Джек Дениелс (голяшка)
(65, 'pair_red_wine'), (65, 'pair_beer_dark'), (65, 'role_main'), (65, 'role_main_filling'), (65, 'occasion_celebration'), (65, 'vibe_hearty'), (65, 'vibe_warming'),
-- 66. Свиная рулька
(66, 'pair_beer_dark'), (66, 'pair_red_wine'), (66, 'role_main'), (66, 'role_main_filling'), (66, 'occasion_celebration'), (66, 'vibe_hearty'),
-- 78. Утка конфи с печёными яблоками
(78, 'pair_red_wine'), (78, 'pair_white_wine'), (78, 'role_main'), (78, 'occasion_date'), (78, 'occasion_celebration'), (78, 'vibe_hearty'),
-- 79. Шашлык из говядины
(79, 'pair_red_wine'), (79, 'pair_beer_dark'), (79, 'role_main'), (79, 'role_main_filling'), (79, 'role_share'), (79, 'vibe_hearty'),
-- 80. Щёки с картофельным пюре (томлёные)
(80, 'pair_red_wine'), (80, 'role_main'), (80, 'role_main_filling'), (80, 'vibe_warming'), (80, 'vibe_hearty'),
-- 172. Рамен с курицей
(172, 'pair_white_wine'), (172, 'pair_lemonade'), (172, 'role_main'), (172, 'vibe_warming'),
-- 175. Пад-тай с курицей (рисовая лапша)
(175, 'pair_white_wine'), (175, 'pair_lemonade'), (175, 'role_main'), (175, 'vibe_warming'),
-- 180. Овощное рагу по-провански (рататуй)
(180, 'pair_white_wine'), (180, 'pair_red_wine'), (180, 'role_main'), (180, 'vibe_light'), (180, 'vibe_warming'),

-- ── Морепродукты ───────────────────────────────────────────────────────────
-- 60. Дорадо (уже было засеяно вручную — будет no-op через ON CONFLICT)
(60, 'pair_white_wine'), (60, 'pair_sparkling'), (60, 'role_main'), (60, 'occasion_date'), (60, 'vibe_light'),
-- 61. Креветки Пиль-Пиль (чеснок, паприка)
(61, 'pair_white_wine'), (61, 'pair_sparkling'), (61, 'role_main'), (61, 'role_aperitif'), (61, 'occasion_date'), (61, 'vibe_light'),
-- 68. Сибас
(68, 'pair_white_wine'), (68, 'pair_sparkling'), (68, 'role_main'), (68, 'occasion_date'), (68, 'vibe_light'),
-- 71. Стейк из форели
(71, 'pair_white_wine'), (71, 'pair_sparkling'), (71, 'role_main'), (71, 'occasion_date'), (71, 'vibe_light'),
-- 77. Тартар из форели (авокадо, огурец)
(77, 'pair_white_wine'), (77, 'pair_sparkling'), (77, 'role_aperitif'), (77, 'role_starter'), (77, 'occasion_date'), (77, 'vibe_light'), (77, 'vibe_refreshing'),
-- 185. Лосось на пару с овощами
(185, 'pair_white_wine'), (185, 'role_main'), (185, 'occasion_business_lunch'), (185, 'vibe_light'),
-- 191. Морское плато (на 3-4 человек, креветки, мидии)
(191, 'pair_white_wine'), (191, 'pair_sparkling'), (191, 'role_share'), (191, 'role_main'), (191, 'occasion_celebration'), (191, 'occasion_date'), (191, 'vibe_light'),

-- ── Гарниры ────────────────────────────────────────────────────────────────
-- 91. Брокколи
(91, 'pair_white_wine'), (91, 'role_side'), (91, 'vibe_light'),
-- 92. Картофель батат фри
(92, 'pair_beer_light'), (92, 'role_side'), (92, 'occasion_kids'),
-- 93. Картофель Диппер (соус Jack Daniel's)
(93, 'pair_beer_dark'), (93, 'pair_red_wine'), (93, 'role_side'), (93, 'vibe_hearty'),
-- 94. Картофель по-деревенски
(94, 'pair_beer_light'), (94, 'pair_red_wine'), (94, 'role_side'), (94, 'vibe_hearty'),
-- 95. Картофель фри
(95, 'pair_beer_light'), (95, 'role_side'), (95, 'occasion_kids'),
-- 96. Картофель фри с пармезаном (трюфель)
(96, 'pair_red_wine'), (96, 'pair_white_wine'), (96, 'role_side'), (96, 'vibe_hearty'),
-- 97. Картофельное пюре
(97, 'pair_red_wine'), (97, 'role_side'), (97, 'occasion_kids'), (97, 'vibe_hearty'),
-- 98. Овощи гриль (цукини, баклажан)
(98, 'pair_white_wine'), (98, 'pair_red_wine'), (98, 'role_side'), (98, 'vibe_light'),
-- 99. Рис с овощами (wok)
(99, 'pair_white_wine'), (99, 'role_side'), (99, 'vibe_light'),

-- ── Соусы (все role_side, остальные оси нерелевантны) ──────────────────────
(100, 'role_side'), (101, 'role_side'), (102, 'role_side'), (103, 'role_side'),
(104, 'role_side'), (105, 'role_side'), (105, 'occasion_kids'),
(106, 'role_side'), (107, 'role_side'), (108, 'role_side'),
(109, 'role_side'), (110, 'role_side'), (111, 'role_side'),

-- ── Десерты ────────────────────────────────────────────────────────────────
-- 112. Медовик
(112, 'pair_tea'), (112, 'pair_coffee'), (112, 'role_finish'), (112, 'vibe_warming'),
-- 113. Мороженое
(113, 'pair_coffee'), (113, 'role_finish'), (113, 'occasion_kids'), (113, 'vibe_refreshing'), (113, 'vibe_light'),
-- 114. Панна-Котта с клубничным
(114, 'pair_sparkling'), (114, 'pair_coffee'), (114, 'role_finish'), (114, 'occasion_date'), (114, 'vibe_light'),
-- 115. Фисташковый рулет
(115, 'pair_tea'), (115, 'pair_coffee'), (115, 'pair_sparkling'), (115, 'role_finish'), (115, 'occasion_date'), (115, 'vibe_light'),
-- 116. Чизкейк баскский
(116, 'pair_coffee'), (116, 'pair_tea'), (116, 'role_finish'), (116, 'vibe_warming'),
-- 117. Шоколадный Фондан (тёплый, жидкая сердцевина)
(117, 'pair_coffee'), (117, 'pair_red_wine'), (117, 'role_finish'), (117, 'occasion_date'), (117, 'vibe_warming'), (117, 'vibe_hearty'),
-- 181. Гранола с фруктами и кокосовым молоком
(181, 'pair_coffee'), (181, 'pair_tea'), (181, 'role_finish'), (181, 'occasion_breakfast'), (181, 'vibe_light'), (181, 'vibe_refreshing'),
-- 186. Чизкейк Нью-Йорк
(186, 'pair_coffee'), (186, 'pair_tea'), (186, 'role_finish'), (186, 'vibe_hearty'),
-- 187. Тирамису (маскарпоне, кофе)
(187, 'pair_coffee'), (187, 'role_finish'), (187, 'occasion_date'), (187, 'vibe_hearty'),
-- 188. Эклер с заварным кремом
(188, 'pair_coffee'), (188, 'pair_tea'), (188, 'role_finish'), (188, 'occasion_kids'), (188, 'vibe_light'),

-- ── Напитки (кофе/чай — role_finish; лимонады/морсы — role_aperitif) ──────
-- Кофейная карта
(118, 'role_finish'), (118, 'role_aperitif'), (118, 'vibe_refreshing'), (118, 'occasion_breakfast'),
(119, 'role_finish'), (119, 'role_aperitif'), (119, 'occasion_business_lunch'), (119, 'occasion_breakfast'), (119, 'vibe_warming'),
(120, 'role_finish'), (120, 'vibe_refreshing'),
(121, 'role_finish'), (121, 'vibe_refreshing'),
(122, 'role_finish'), (122, 'vibe_refreshing'),
(123, 'role_finish'), (123, 'occasion_kids'), (123, 'vibe_warming'),
(124, 'role_finish'), (124, 'occasion_business_lunch'), (124, 'vibe_warming'),
(125, 'role_finish'), (125, 'occasion_breakfast'), (125, 'vibe_warming'),
(126, 'role_finish'), (126, 'occasion_breakfast'), (126, 'vibe_warming'),
(127, 'role_finish'), (127, 'vibe_warming'),
(128, 'role_finish'), (128, 'occasion_breakfast'), (128, 'vibe_warming'),
(129, 'role_finish'), (129, 'vibe_warming'),
(130, 'role_finish'), (130, 'vibe_warming'),
(131, 'role_finish'), (131, 'occasion_kids'), (131, 'vibe_warming'),
(132, 'role_finish'), (132, 'occasion_breakfast'), (132, 'vibe_warming'),
(133, 'role_finish'), (133, 'occasion_breakfast'), (133, 'vibe_warming'),
-- Чаи
(134, 'role_finish'), (134, 'vibe_warming'),
(135, 'role_finish'), (135, 'occasion_kids'), (135, 'vibe_warming'),
(136, 'role_finish'), (136, 'vibe_warming'),
(137, 'role_finish'), (137, 'vibe_warming'), (137, 'vibe_refreshing'),
(138, 'role_finish'), (138, 'vibe_refreshing'),
(139, 'role_finish'), (139, 'vibe_warming'),
(140, 'role_finish'), (140, 'vibe_warming'),
(141, 'role_finish'), (141, 'occasion_kids'), (141, 'vibe_warming'),
(142, 'role_finish'), (142, 'vibe_warming'),
(143, 'role_finish'), (143, 'vibe_warming'),
(144, 'role_finish'), (144, 'vibe_warming'),
(145, 'role_finish'), (145, 'vibe_warming'),
-- Б/А коктейли и лимонады
(146, 'role_aperitif'), (146, 'vibe_refreshing'), (146, 'occasion_date'),
(147, 'role_aperitif'), (147, 'occasion_kids'), (147, 'vibe_refreshing'),
(148, 'role_aperitif'), (148, 'occasion_kids'), (148, 'vibe_refreshing'),
(149, 'role_aperitif'), (149, 'occasion_kids'), (149, 'vibe_refreshing'),
(150, 'role_aperitif'), (150, 'occasion_kids'), (150, 'vibe_refreshing'),
(151, 'role_aperitif'), (151, 'occasion_kids'), (151, 'vibe_refreshing'),
(152, 'role_aperitif'), (152, 'occasion_kids'), (152, 'vibe_refreshing'),
(153, 'role_aperitif'), (153, 'occasion_kids'), (153, 'vibe_refreshing'),
(154, 'role_aperitif'), (154, 'occasion_kids'), (154, 'occasion_breakfast'), (154, 'vibe_refreshing'),
(155, 'role_aperitif'), (155, 'vibe_refreshing'), (155, 'occasion_date'),
(156, 'role_aperitif'), (156, 'vibe_refreshing'), (156, 'occasion_date'),
(157, 'role_aperitif'), (157, 'occasion_kids'), (157, 'vibe_refreshing'),
(158, 'role_aperitif'), (158, 'role_finish'), (158, 'vibe_warming'),
(168, 'role_aperitif'), (168, 'occasion_date'), (168, 'vibe_refreshing'),
(169, 'role_aperitif'), (169, 'occasion_date'), (169, 'vibe_refreshing'),

-- ── Алкоголь (всё безалкогольное по факту: б/а вина, пиво, эль, сидр) ──────
(159, 'role_aperitif'), (159, 'occasion_date'), (159, 'vibe_refreshing'),
(160, 'role_aperitif'), (160, 'occasion_date'), (160, 'occasion_celebration'), (160, 'vibe_refreshing'),
(161, 'role_aperitif'), (161, 'occasion_date'), (161, 'vibe_refreshing'),
(162, 'role_aperitif'), (162, 'vibe_refreshing'),
(163, 'role_aperitif'), (163, 'vibe_refreshing'),
(164, 'role_aperitif'), (164, 'vibe_hearty'), (164, 'vibe_warming'),
(165, 'role_aperitif'), (165, 'vibe_refreshing'), (165, 'occasion_date'),
(166, 'role_aperitif'), (166, 'vibe_warming'),
(167, 'role_aperitif'), (167, 'vibe_refreshing')

ON CONFLICT (dish_id, tag_slug) DO NOTHING;

COMMIT;

-- Контроль: сколько блюд покрыто, сколько средне тегов на блюдо.
SELECT
    COUNT(DISTINCT dish_id) AS dishes_with_tags,
    COUNT(*) AS total_links,
    ROUND(AVG(c), 2) AS avg_tags_per_dish
FROM (
    SELECT dish_id, COUNT(*) AS c
    FROM dish_pairing_tags
    GROUP BY dish_id
) t;
