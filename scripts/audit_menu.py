"""Read takeout xlsx, normalize categories, apply heuristics, enrich, emit JSON + audit."""

from __future__ import annotations

import hashlib
import io
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path

import openpyxl

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")

ROOT = Path(__file__).resolve().parents[1]
XLSX = ROOT / "thebull_takeout_menu_kbju_fixed_images.xlsx"
OUT_JSON = ROOT / "seed" / "menu.json"
OUT_AUDIT = ROOT / "seed" / "audit.md"


# ----- categories -----

CATEGORIES = [
    ("Закуски", 10),
    ("Салаты", 20),
    ("Супы", 30),
    ("Пицца и паста", 40),
    ("Бургеры", 50),
    ("Стейки", 60),
    ("Гриль (хоспер)", 70),
    ("Горячее — мясо", 80),
    ("Горячее — рыба и морепродукты", 90),
    ("Гарниры", 100),
    ("Соусы", 110),
    ("Десерты", 120),
    ("Напитки безалкогольные", 130),
    ("Напитки алкогольные", 140),
]

# rules by ORIGINAL category, optionally narrowed by name keywords
RAW_CAT_MAP = {
    "Закуски": "Закуски",
    "Салаты": "Салаты",
    "Супы": "Супы",
    "Бургеры": "Бургеры",
    "Хоспер": "Гриль (хоспер)",
    "Гарниры": "Гарниры",
    "Соусы": "Соусы",
    "Десерты": "Десерты",
    "Римская пицца": "Пицца и паста",
    "Кофе": "Напитки безалкогольные",
    "Кофе на альтернативном молоке": "Напитки безалкогольные",
    "Чай": "Напитки безалкогольные",
    "Лимонады": "Напитки безалкогольные",
    "Горячие коктейли": "Напитки безалкогольные",
    "The Бык": "Напитки безалкогольные",
    "Пиво": "Напитки алкогольные",
    "Вино Б/А": "Напитки алкогольные",
    "Альтернативные стейки": "Стейки",
    "Prime Beef": "Стейки",
}


def map_category(orig: str, name: str) -> str:
    """Pick the new category. 'Prime' and 'Разное' are routed by item name."""
    if orig in RAW_CAT_MAP:
        return RAW_CAT_MAP[orig]

    n = (name or "").lower()
    fish = ("дорадо", "сибас", "форел", "лосос", "креветк", "тунец", "осьминог", "мидии", "кальмар", "судак")
    pasta_pizza = ("паст", "карбонар", "болоньезе", "ризотт")
    starters = ("тартар", "фокачч", "хлебная корзина", "карпаччо")
    light_meat = ("щёки", "щеки", "рулька", "рульк")

    if orig == "Prime":
        if any(k in n for k in fish):
            return "Горячее — рыба и морепродукты"
        if any(k in n for k in pasta_pizza):
            return "Пицца и паста"
        if any(k in n for k in starters):
            return "Закуски"
        if "стейк" in n or "томагавк" in n or "рибай" in n or "филе миньон" in n:
            return "Стейки"
        if "утка" in n or "рябчик" in n or "шашлык" in n or any(k in n for k in light_meat):
            return "Горячее — мясо"
        return "Горячее — мясо"

    if orig == "Разное":
        if any(k in n for k in ("фокачч", "хлебная корзина")):
            return "Закуски"
        return "Горячее — мясо"

    return "Закуски"  # fallback — shouldn't hit


# ----- cuisine -----

CUISINE_DEFAULT_BY_CATEGORY = {
    "Стейки": "american",
    "Гриль (хоспер)": "american",
    "Бургеры": "american",
    "Пицца и паста": "italian",
    "Соусы": "european",
    "Гарниры": "european",
    "Салаты": "european",
    "Закуски": "european",
    "Супы": "european",
    "Горячее — мясо": "european",
    "Горячее — рыба и морепродукты": "european",
    "Десерты": "european",
    "Напитки безалкогольные": "european",
    "Напитки алкогольные": "european",
}


CUISINE_RULES = [
    ("japanese", [r"\bролл\b", r"филадельфия", r"калифорния", r"\bмисо\b", r"рамен", r"сашими", r"темпура", r"удон"]),
    ("asian",    [r"том-ям", r"пад-тай", r"тайск", r"сатэй", r"терияки", r"кесадилья"]),
    ("italian",  [r"карбонар", r"болоньезе", r"\bпаста\b", r"\bпенне\b", r"спагетти", r"лазань",
                  r"\bпицц\w*", r"вителло", r"тирамису", r"ризотт", r"\bпесто\b", r"пармезан"]),
    ("french",   [r"\bэклер\b", r"круассан", r"\bконфи\b", r"ратат"]),
    ("russian",  [r"\bборщ\b", r"пельмен", r"вареник", r"\bблин\w*", r"оливье", r"винегрет", r"\bщи\b"]),
]
CUISINE_RULES_COMPILED = [(c, [re.compile(p) for p in pats]) for c, pats in CUISINE_RULES]


def detect_cuisine(category: str, name: str, composition: str) -> str:
    text = f"{name} {composition}".lower()
    # short-circuit для ближневосточных блюд: их 'паста тахини' иначе ловится italian
    if any(k in text for k in ("тахини", "бабагануш", "хумус")):
        return CUISINE_DEFAULT_BY_CATEGORY.get(category, "european")
    for cuisine, pats in CUISINE_RULES_COMPILED:
        if any(p.search(text) for p in pats):
            return cuisine
    return CUISINE_DEFAULT_BY_CATEGORY.get(category, "european")


# ----- allergens -----

ALLERGEN_RULES = [
    ("dairy",     ("молок", "сыр", "сметан", "сливк", "творог", "масло сливоч", "масл слив",
                   "моцарелл", "пармезан", "йогурт", "кефир", "фета", "рикотт",
                   "маскарпоне", "горгондзол", "буррат", "чеддер")),
    ("eggs",      ("яйц", "яичн", "майонез")),
    ("shellfish", ("креветк", "мидии", "осьминог", "кальмар", "омар", "лангуст", "краб")),
    ("fish",      ("лосос", "тунец", "дорадо", "сибас", "форел", "икра", "анчоус", "тунца")),
    ("gluten",    ("мука", "хлеб", "пита", "пицц", "паст", "макарон", "спагетти", "пенне",
                   "гриссини", "сухари", "панировк", "булочк", "тортилья", "кесадилья",
                   "фокачч", "круассан", "тесто", "багет", "брускетт")),
    ("peanuts",   ("арахис",)),
    ("nuts",      ("орех", "кедров", "грецк", "миндал", "фундук", "фисташ", "кешью", "пекан")),
    ("soy",       ("соев", "тофу", "мисо ", "соевый соус", "соус 1000")),
    ("sesame",    ("кунжут", "тахини", "sesame")),
    ("mustard",   ("горчиц",)),
    ("celery",    ("сельдер",)),
]


def detect_allergens(composition: str, name: str) -> list[str]:
    text = f"{name} {composition}".lower()
    found = []
    for code, kws in ALLERGEN_RULES:
        if any(k in text for k in kws):
            found.append(code)
    # sanity: 'арахис' shouldn't also tag 'nuts' if there's no other treenut
    if "peanuts" in found and "nuts" in found:
        treenut_kws = ("кедров", "грецк", "миндал", "фундук", "фисташ", "кешью", "пекан")
        if not any(k in text for k in treenut_kws):
            found.remove("nuts")
    return found


# ----- dietary -----

MEAT_KWS = ("говядин", "свинин", "свини", "баран", "утк", "куриц", "курин", "индейк", "индюш",
            "рябчик", "котлет", "фарш", "ветчин", "салями", "чоризо", "пастрами",
            "ростбиф", "бекон", "сосиск", "колбас", "кебаб", "шашлык", "тартар",
            "бургер", "стейк", "бефстроганов", "брискет", "пиканья", "ребр")
FISH_KWS = ("лосос", "тунец", "дорадо", "сибас", "форел", "икра", "анчоус",
            "креветк", "мидии", "осьминог", "кальмар", "омар", "краб")
ALCOHOL_KWS = ("алкоголь", "ром", "виски", "водк", "коньяк", "бренди", "ликер", "ликёр",
               "вино ", "пиво ", "jack daniel", "абсент", "текила", "джин", "куантро")
PORK_KWS = ("свинин", "свини", "бекон", "ветчин", "пастрами", "салями", "чоризо",
            "колбас", "сосиск", "ребра «барбекю»", "ребра «терияки»",
            "ребра jack daniel", "медальоны из свинины", "шашлык из свиной шеи",
            "томагавк", "свиная рулька", "свиные колбаски")


def detect_dietary(composition: str, name: str, allergens: list[str], halal_explicit: bool = False) -> list[str]:
    text = f"{name} {composition}".lower()
    has_meat = any(k in text for k in MEAT_KWS)
    has_fish = any(k in text for k in FISH_KWS)
    out = []
    if not has_meat and not has_fish:
        out.append("vegetarian")
        if "dairy" not in allergens and "eggs" not in allergens:
            out.append("vegan")
    if "gluten" not in allergens and "wheat" not in text:
        out.append("gluten_free")
    if "dairy" not in allergens:
        out.append("lactose_free")
    if halal_explicit:
        has_pork = any(k in text for k in PORK_KWS)
        has_alc = any(k in text for k in ALCOHOL_KWS)
        if not has_pork and not has_alc:
            out.append("halal")
    return out


# ----- tags -----

SPICY_KWS = ("чили", "халапеньо", "sriracha", "шрирача", "острый", "острая", "острое",
             "пиль-пиль", "кайен", "том-ям", "острых", "сладкий чили", "тайск")


def detect_tags(category: str, original_category: str, name: str, composition: str,
                price_minor: int, weight_g: int | None, calories: int | None,
                is_synthetic: bool) -> list[str]:
    text = f"{name} {composition}".lower()
    tags = []

    if original_category == "Кофе" or original_category == "Кофе на альтернативном молоке":
        tags.append("coffee")
    if original_category == "Чай":
        tags.append("tea")
    if original_category == "Лимонады":
        tags.append("lemonade")
    if original_category == "Пиво":
        tags.append("beer")
    if original_category == "Вино Б/А":
        tags.append("wine")

    if any(k in text for k in SPICY_KWS):
        tags.append("spicy")

    HIT_NAMES = {
        "рибай блэк ангус", "стейк шато бриан", "стейк филе миньон", "стейк рибай (трава)",
        "утка конфи с печеными яблоками", "бургер the бык", "цезарь с курицей", "цезарь с креветками",
        "греческий салат", "паста с томлёной щекой",
    }
    if name.lower() in HIT_NAMES:
        tags.append("hit")

    CHEF_NAMES = HIT_NAMES | {
        "брискет", "ребра кальби", "стейк стриплойн (зерно)", "стейк бавет (зерно)",
        "тартар из говядины", "вителло тоннато", "карбонара",
    }
    if name.lower() in CHEF_NAMES:
        tags.append("chef")

    if weight_g is not None and weight_g >= 400:
        tags.append("big")
    SHARE_KWS = ("плато", "сет", "для компании", "на двоих", "доска")
    if weight_g is not None and weight_g >= 500 and any(k in name.lower() for k in SHARE_KWS):
        tags.append("share")
    if calories is not None and calories <= 350 and (weight_g is None or weight_g < 400):
        if category not in ("Соусы", "Гарниры", "Напитки безалкогольные", "Напитки алкогольные"):
            tags.append("light")

    if is_synthetic:
        tags.append("new")

    return list(dict.fromkeys(tags))  # dedupe, preserve order


# ----- weight parser -----

WEIGHT_NUM_RE = re.compile(r"(\d+)")


def parse_weight(raw: str | None) -> int | None:
    if not raw:
        return None
    if "мл" in raw.lower():
        return None
    nums = [int(n) for n in WEIGHT_NUM_RE.findall(raw)]
    if not nums:
        return None
    # forms like "1 шт/45 г": skip leading 1-2 digit if "шт" present before it
    if "шт" in raw.lower():
        for n in nums:
            if n > 5:
                return n
        return None
    # forms like "260/40 г" — main portion = first; "/40" is sauce
    return nums[0] if nums[0] > 0 else None


# ----- synthetic dishes -----

SYNTHETIC: list[dict] = [
    # ===== japanese / asian =====
    {
        "name": "Том-ям с креветками", "category": "Супы", "cuisine": "asian",
        "description": "Классический тайский острый суп с креветками, грибами шиитаке и кокосовым молоком.",
        "composition": "Креветки тигровые, кокосовое молоко, грибы шиитаке, лимонная трава, галангал, листья каффир-лайма, лайм, чили, рыбный соус, кинза.",
        "price": 690, "weight_g": 350, "kcal": 280, "p": 18.0, "f": 16.0, "c": 12.0,
        "halal": False,
    },
    {
        "name": "Мисо-суп с тофу", "category": "Супы", "cuisine": "japanese",
        "description": "Лёгкий японский бульон даси с пастой мисо, кубиками тофу и нори.",
        "composition": "Бульон даси, паста мисо, тофу, водоросли вакамэ, зелёный лук, нори.",
        "price": 350, "weight_g": 300, "kcal": 95, "p": 7.0, "f": 4.0, "c": 6.0,
        "halal": True,
    },
    {
        "name": "Рамен с курицей", "category": "Горячее — мясо", "cuisine": "japanese",
        "description": "Японская лапша в курином бульоне с маринованным яйцом и побегами бамбука.",
        "composition": "Куриный бульон, лапша рамен, филе курицы, маринованное яйцо, побеги бамбука менма, зелёный лук, нори, кунжут.",
        "price": 590, "weight_g": 450, "kcal": 520, "p": 28.0, "f": 14.0, "c": 65.0,
        "halal": True,
    },
    {
        "name": "Ролл Филадельфия", "category": "Закуски", "cuisine": "japanese",
        "description": "Классический ролл с лососем и сливочным сыром.",
        "composition": "Лосось слабосолёный, сливочный сыр Филадельфия, рис, нори, огурец, авокадо.",
        "price": 690, "weight_g": 230, "kcal": 380, "p": 18.0, "f": 18.0, "c": 36.0,
        "halal": False,
    },
    {
        "name": "Ролл Калифорния", "category": "Закуски", "cuisine": "japanese",
        "description": "Ролл с крабом, авокадо и икрой тобико.",
        "composition": "Краб, авокадо, рис, нори, икра тобико, огурец, майонез.",
        "price": 590, "weight_g": 220, "kcal": 320, "p": 12.0, "f": 14.0, "c": 36.0,
        "halal": False,
    },
    {
        "name": "Пад-тай с курицей", "category": "Горячее — мясо", "cuisine": "asian",
        "description": "Тайская рисовая лапша вок с курицей, арахисом и яйцом.",
        "composition": "Лапша рисовая, филе курицы, ростки сои, арахис, яйцо, лук репчатый, тамаринд, лайм, рыбный соус, чили.",
        "price": 620, "weight_g": 380, "kcal": 580, "p": 24.0, "f": 22.0, "c": 70.0,
        "halal": False,
    },

    # ===== italian extras =====
    {
        "name": "Карбонара", "category": "Пицца и паста", "cuisine": "italian",
        "description": "Спагетти в классическом соусе из желтка, гуанчале и пекорино.",
        "composition": "Спагетти, гуанчале, желток куриный, сыр пекорино романо, чёрный перец.",
        "price": 590, "weight_g": 320, "kcal": 560, "p": 24.0, "f": 28.0, "c": 48.0,
        "halal": False,
    },
    {
        "name": "Паста Аррабиата", "category": "Пицца и паста", "cuisine": "italian",
        "description": "Пенне в остром томатном соусе с чесноком и чили.",
        "composition": "Пенне, томаты в собственном соку, чеснок, чили, оливковое масло, петрушка.",
        "price": 490, "weight_g": 350, "kcal": 420, "p": 12.0, "f": 14.0, "c": 60.0,
        "halal": True,
    },
    {
        "name": "Ризотто с белыми грибами", "category": "Пицца и паста", "cuisine": "italian",
        "description": "Кремовое ризотто с белыми грибами и пармезаном.",
        "composition": "Рис арборио, белые грибы, лук репчатый, белое сухое вино, пармезан, сливочное масло, петрушка.",
        "price": 690, "weight_g": 320, "kcal": 480, "p": 14.0, "f": 18.0, "c": 60.0,
        "halal": False,
    },

    # ===== vegan / vegetarian =====
    {
        "name": "Боул с киноа и авокадо", "category": "Салаты", "cuisine": "european",
        "description": "Сытный веганский боул с киноа, авокадо, нутом и табуле.",
        "composition": "Киноа, авокадо, нут, помидоры черри, огурец, петрушка, кинза, лимон, оливковое масло.",
        "price": 520, "weight_g": 320, "kcal": 380, "p": 12.0, "f": 14.0, "c": 50.0,
        "halal": True,
    },
    {
        "name": "Овощное рагу по-провански", "category": "Горячее — мясо", "cuisine": "french",
        "description": "Рататуй из овощей, томлёный с прованскими травами.",
        "composition": "Баклажаны, кабачки, болгарский перец, помидоры, лук, чеснок, оливковое масло, тимьян, базилик.",
        "price": 420, "weight_g": 300, "kcal": 220, "p": 5.0, "f": 12.0, "c": 26.0,
        "halal": True,
    },
    {
        "name": "Гранола с фруктами и кокосовым молоком", "category": "Десерты", "cuisine": "european",
        "description": "Веганский десерт-завтрак: домашняя гранола, ягоды, кокосовое молоко.",
        "composition": "Овсяные хлопья, мёд (опционально кленовый сироп), миндаль, кокосовая стружка, банан, клубника, голубика, кокосовое молоко.",
        "price": 390, "weight_g": 250, "kcal": 380, "p": 8.0, "f": 16.0, "c": 50.0,
        "halal": True,
    },
    {
        "name": "Хумус с овощами и питой", "category": "Закуски", "cuisine": "european",
        "description": "Восточная закуска: классический хумус с оливковым маслом, овощи и тёплая пита.",
        "composition": "Нут, тахини, чеснок, лимон, оливковое масло, паприка, морковь, огурец, болгарский перец, пита.",
        "price": 390, "weight_g": 280, "kcal": 360, "p": 12.0, "f": 18.0, "c": 38.0,
        "halal": True,
    },

    # ===== halal-confident =====
    {
        "name": "Шашлык из ягнёнка халяль", "category": "Гриль (хоспер)", "cuisine": "asian",
        "description": "Маринованная баранина на шпажках, приготовленная на углях.",
        "composition": "Мякоть ягнёнка, лук репчатый, кориандр, зира, перец чёрный, оливковое масло.",
        "price": 750, "weight_g": 250, "kcal": 480, "p": 32.0, "f": 36.0, "c": 4.0,
        "halal": True,
    },
    {
        "name": "Куриный шашлык в маринаде sriracha", "category": "Гриль (хоспер)", "cuisine": "asian",
        "description": "Острые шпажки куриного бедра в маринаде sriracha и чесноке.",
        "composition": "Бедро куриное, чеснок, sriracha, мёд, соевый соус, имбирь, лайм.",
        "price": 490, "weight_g": 230, "kcal": 380, "p": 28.0, "f": 18.0, "c": 18.0,
        "halal": True,
    },

    # ===== light fish =====
    {
        "name": "Лосось на пару с овощами", "category": "Горячее — рыба и морепродукты", "cuisine": "european",
        "description": "Лёгкий стейк лосося на пару с сезонными овощами.",
        "composition": "Лосось, брокколи, морковь, цветная капуста, лимон, оливковое масло, морская соль.",
        "price": 890, "weight_g": 280, "kcal": 320, "p": 30.0, "f": 18.0, "c": 12.0,
        "halal": False,
    },

    # ===== desserts =====
    {
        "name": "Чизкейк Нью-Йорк", "category": "Десерты", "cuisine": "american",
        "description": "Классический чизкейк на песочной основе с ягодным соусом.",
        "composition": "Сливочный сыр, песочное тесто, яйца, сахар, ваниль, соус из ягод.",
        "price": 390, "weight_g": 140, "kcal": 420, "p": 7.0, "f": 26.0, "c": 38.0,
        "halal": False,
    },
    {
        "name": "Тирамису", "category": "Десерты", "cuisine": "italian",
        "description": "Воздушный итальянский десерт с маскарпоне и кофейным сиропом.",
        "composition": "Маскарпоне, печенье савоярди, кофе эспрессо, какао, желток, сахар.",
        "price": 420, "weight_g": 140, "kcal": 380, "p": 6.0, "f": 22.0, "c": 38.0,
        "halal": False,
    },
    {
        "name": "Эклер с заварным кремом", "category": "Десерты", "cuisine": "french",
        "description": "Французский эклер с классическим заварным кремом.",
        "composition": "Заварное тесто, заварной крем, ванильный сахар, шоколадная глазурь, яйца, молоко.",
        "price": 290, "weight_g": 100, "kcal": 320, "p": 5.0, "f": 14.0, "c": 42.0,
        "halal": True,
    },

    # ===== share platters =====
    {
        "name": "Большое мясное плато для компании", "category": "Закуски", "cuisine": "european",
        "description": "Ассорти мясных деликатесов на компанию из 4-6 человек.",
        "composition": "Пармская ветчина, пастрами, салями чоризо, ростбиф, вяленые томаты, гриссини, оливки, корнишоны, дижонская горчица.",
        "price": 2200, "weight_g": 700, "kcal": 1450, "p": 95.0, "f": 105.0, "c": 35.0,
        "halal": False,
    },
    {
        "name": "Сырная доска для компании", "category": "Закуски", "cuisine": "french",
        "description": "Ассорти сыров с медом, орехами и виноградом для большой компании.",
        "composition": "Пармезан, дор-блю, бри, чеддер, моцарелла, мёд, грецкий орех, миндаль, виноград, гриссини.",
        "price": 1900, "weight_g": 600, "kcal": 1620, "p": 80.0, "f": 130.0, "c": 28.0,
        "halal": False,
    },
    {
        "name": "Морское плато", "category": "Горячее — рыба и морепродукты", "cuisine": "european",
        "description": "Ассорти морепродуктов с лимоном и тремя соусами на 3-4 персоны.",
        "composition": "Тигровые креветки, мидии в раковине, осьминог отварной, стейк лосося, лимон, соус айоли, соус коктейль, соус тартар.",
        "price": 3500, "weight_g": 800, "kcal": 1280, "p": 130.0, "f": 60.0, "c": 18.0,
        "halal": False,
    },
    {
        "name": "Стейк-сет на двоих", "category": "Стейки", "cuisine": "american",
        "description": "Два премиальных стейка (рибай и бавет) с тремя соусами для двоих.",
        "composition": "Стейк рибай блэк ангус, стейк бавет, гарнир из картофеля бэби, соус демиглас, соус перечный, соус чимичурри, морская соль, розмарин.",
        "price": 3200, "weight_g": 600, "kcal": 2100, "p": 160.0, "f": 140.0, "c": 30.0,
        "halal": False,
    },
    {
        "name": "Закусочный сет на компанию", "category": "Закуски", "cuisine": "american",
        "description": "Большой сет горячих закусок к пиву на 4 человек.",
        "composition": "Куриные крылья BBQ, чесночные гренки, картофель фри, луковые кольца, кесадилья с курицей, соус 1000 островов, соус блю чиз, соус сладкий чили.",
        "price": 1800, "weight_g": 800, "kcal": 2350, "p": 75.0, "f": 130.0, "c": 145.0,
        "halal": False,
    },
]


# ----- main pipeline -----

def image_key(name: str) -> str:
    """Stable ASCII S3 key for a dish; key is `dishes/<md5-12>.jpg`."""
    h = hashlib.md5(name.encode("utf-8")).hexdigest()[:12]
    return f"dishes/{h}.jpg"


def main() -> None:
    wb = openpyxl.load_workbook(XLSX, data_only=True)
    ws = wb["Меню THE БЫК"]
    rows = list(ws.iter_rows(min_row=2, values_only=True))

    seen_names: set[str] = set()
    dishes: list[dict] = []

    for row in rows:
        if not row or row[0] is None:
            continue
        original_cat, name, composition, weight_raw, price, status, p, f, c, kcal, _kbju, image_url, _src = row[:13]
        if not name:
            continue
        if name in seen_names:
            continue  # skip duplicate wines
        seen_names.add(name)

        composition = composition or ""
        category = map_category(original_cat, name)
        cuisine = detect_cuisine(category, name, composition)
        weight_g = parse_weight(weight_raw)
        price_minor = int(price) * 100 if price is not None else None
        kcal_v = int(kcal) if kcal is not None else None
        is_available = not (status and str(status).lower().startswith("нет"))

        allergens = detect_allergens(composition, name)
        dietary = detect_dietary(composition, name, allergens, halal_explicit=False)
        tags = detect_tags(
            category, original_cat, name, composition,
            price_minor or 0, weight_g, kcal_v, is_synthetic=False,
        )

        if image_url:
            url_str = str(image_url).lower()
            # placeholder / inline data-URI / svg lazy-load — не картинки, дропаем
            if "placeholder" in url_str or url_str.startswith("data:") or "svg+xml" in url_str:
                image_url = ""

        dishes.append({
            "name": name,
            "description": "",
            "composition": composition,
            "image_url_external": image_url or "",
            "image_key": image_key(name) if image_url else "",
            "price_minor": price_minor or 0,
            "currency": "RUB",
            "calories_kcal": kcal_v,
            "protein_g": float(p) if p is not None else None,
            "fat_g": float(f) if f is not None else None,
            "carbs_g": float(c) if c is not None else None,
            "portion_weight_g": weight_g,
            "cuisine": cuisine,
            "category": category,
            "allergens": allergens,
            "dietary": dietary,
            "tag_slugs": tags,
            "is_available": is_available,
            "synthetic": False,
        })

    # add synthetic
    for s in SYNTHETIC:
        name = s["name"]
        if name in seen_names:
            continue
        seen_names.add(name)
        comp = s["composition"]
        cat = s["category"]
        allergens = detect_allergens(comp, name)
        dietary = detect_dietary(comp, name, allergens, halal_explicit=s.get("halal", False))
        tags = detect_tags(
            cat, "", name, comp,
            s["price"] * 100, s["weight_g"], s["kcal"], is_synthetic=True,
        )
        dishes.append({
            "name": name,
            "description": s.get("description", ""),
            "composition": comp,
            "image_url_external": "",
            "image_key": "",
            "price_minor": s["price"] * 100,
            "currency": "RUB",
            "calories_kcal": s["kcal"],
            "protein_g": s["p"],
            "fat_g": s["f"],
            "carbs_g": s["c"],
            "portion_weight_g": s["weight_g"],
            "cuisine": s["cuisine"],
            "category": cat,
            "allergens": allergens,
            "dietary": dietary,
            "tag_slugs": tags,
            "is_available": True,
            "synthetic": True,
        })

    tags_meta = [
        {"name": "Хит сезона",       "slug": "hit",      "color": "#FB8C00"},
        {"name": "Новинка",          "slug": "new",      "color": "#43A047"},
        {"name": "Острое",           "slug": "spicy",    "color": "#E53935"},
        {"name": "Шеф рекомендует",  "slug": "chef",     "color": "#9C27B0"},
        {"name": "Большая порция",   "slug": "big",      "color": "#5D4037"},
        {"name": "Для компании",     "slug": "share",    "color": "#00897B"},
        {"name": "Лёгкое",           "slug": "light",    "color": "#0288D1"},
        {"name": "Кофе",             "slug": "coffee",   "color": "#6D4C41"},
        {"name": "Чай",              "slug": "tea",      "color": "#8BC34A"},
        {"name": "Лимонад",          "slug": "lemonade", "color": "#FFC107"},
        {"name": "Вино",             "slug": "wine",     "color": "#7B1FA2"},
        {"name": "Пиво",             "slug": "beer",     "color": "#FFB300"},
    ]

    payload = {
        "categories": [{"name": n, "sort_order": s, "is_available": True} for n, s in CATEGORIES],
        "tags": tags_meta,
        "dishes": dishes,
    }

    OUT_JSON.parent.mkdir(parents=True, exist_ok=True)
    OUT_JSON.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")

    write_audit(payload)


def write_audit(payload: dict) -> None:
    dishes = payload["dishes"]

    by_cat = Counter(d["category"] for d in dishes)
    by_cuisine = Counter(d["cuisine"] for d in dishes)
    avg_price_by_cat: dict[str, float] = {}
    for cat in by_cat:
        prices = [d["price_minor"] for d in dishes if d["category"] == cat and d["price_minor"]]
        if prices:
            avg_price_by_cat[cat] = sum(prices) / len(prices) / 100

    allergen_count = Counter()
    for d in dishes:
        for a in d["allergens"]:
            allergen_count[a] += 1
    no_allergens = sum(1 for d in dishes if not d["allergens"])

    dietary_count = Counter()
    for d in dishes:
        for x in d["dietary"]:
            dietary_count[x] += 1

    tag_count = Counter()
    for d in dishes:
        for t in d["tag_slugs"]:
            tag_count[t] += 1

    no_kbju = sum(1 for d in dishes if d["calories_kcal"] is None)
    no_image = sum(1 for d in dishes if not d["image_url_external"] and not d["synthetic"])
    synthetic = sum(1 for d in dishes if d["synthetic"])
    unavailable = sum(1 for d in dishes if not d["is_available"])

    lines: list[str] = []
    lines.append("# Audit меню после нормализации\n")
    lines.append(f"**Всего блюд:** {len(dishes)} (синтетика: {synthetic}, недоступно: {unavailable})\n")
    lines.append(f"**Без КБЖУ:** {no_kbju}    **Без картинки (из xlsx):** {no_image}\n")

    lines.append("\n## Категории\n")
    lines.append("| Категория | Кол-во | Средняя цена, ₽ |")
    lines.append("|---|---:|---:|")
    for n, _ in CATEGORIES:
        avg = avg_price_by_cat.get(n)
        avg_s = f"{avg:.0f}" if avg else "—"
        lines.append(f"| {n} | {by_cat.get(n, 0)} | {avg_s} |")

    lines.append("\n## Кухни\n")
    lines.append("| Cuisine | Кол-во |")
    lines.append("|---|---:|")
    for c, n in by_cuisine.most_common():
        lines.append(f"| {c} | {n} |")

    lines.append("\n## Аллергены\n")
    lines.append(f"**Без аллергенов** (safe-for-all): {no_allergens}\n")
    lines.append("| Аллерген | Блюд |")
    lines.append("|---|---:|")
    for a, n in allergen_count.most_common():
        lines.append(f"| {a} | {n} |")

    lines.append("\n## Диетические\n")
    lines.append("| Свойство | Блюд |")
    lines.append("|---|---:|")
    for k, n in dietary_count.most_common():
        lines.append(f"| {k} | {n} |")

    lines.append("\n## Теги\n")
    lines.append("| Тег | Блюд |")
    lines.append("|---|---:|")
    for t, n in tag_count.most_common():
        lines.append(f"| {t} | {n} |")

    lines.append("\n## Сэмпл по 1 блюду из каждой категории\n")
    seen = set()
    for d in dishes:
        if d["category"] in seen:
            continue
        seen.add(d["category"])
        lines.append(f"\n### {d['category']} — {d['name']}")
        lines.append(f"- composition: {d['composition'][:120]}{'…' if len(d['composition']) > 120 else ''}")
        lines.append(f"- cuisine: `{d['cuisine']}`  /  price: `{d['price_minor']/100:.0f} ₽`  /  weight: `{d['portion_weight_g']} г`  /  kcal: `{d['calories_kcal']}`")
        lines.append(f"- allergens: `{', '.join(d['allergens']) or '—'}`")
        lines.append(f"- dietary: `{', '.join(d['dietary']) or '—'}`")
        lines.append(f"- tags: `{', '.join(d['tag_slugs']) or '—'}`")

    OUT_AUDIT.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
    print(f"wrote {OUT_JSON.relative_to(ROOT)}")
    print(f"wrote {OUT_AUDIT.relative_to(ROOT)}")
