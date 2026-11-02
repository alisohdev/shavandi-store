export type Size =
  | "XS"
  | "S"
  | "M"
  | "L"
  | "XL"
  | "2XL"
  | "۲۶"
  | "۲۸"
  | "۳۰"
  | "۳۲"
  | "قد ۱۴۰–۱۷۵"
  | "نمایشی";

export type Variant = {
  readonly sku: string;
  readonly size: Size;
  readonly availability: "available" | "sold-out";
};

export type ProductImage = {
  readonly src: string;
  readonly alt: string;
  readonly position: string;
  readonly width: number;
  readonly height: number;
  readonly fit: "cover" | "contain";
  readonly srcSet?: string;
};

export type CatalogProvenance =
  | {
      readonly kind: "sourced";
      readonly sourcePage: string;
      readonly checkedAt: string;
      readonly rights: "confirmed-by-user";
      readonly verifiedFacts: readonly string[];
      readonly demoFacts: readonly string[];
    }
  | {
      readonly kind: "temporary";
      readonly imageCredit: "Unsplash" | "original-illustration";
      readonly note: string;
    };

export type ProductProvenance =
  | CatalogProvenance
  | {
      readonly kind: "local-edit";
      readonly original: CatalogProvenance;
    };

export type Colorway = {
  readonly id: string;
  readonly label: string;
  readonly swatch: string;
  readonly imageNote: string | null;
  readonly images: readonly [ProductImage, ...ProductImage[]];
  readonly variants: readonly [Variant, ...Variant[]];
};

export type Product = {
  readonly id: string;
  readonly slug: string;
  readonly name: string;
  readonly category:
    "تاپ" | "بافت" | "شلوار" | "رویه" | "پیراهن" | "پتو تنپوش" | "ورزشی";
  readonly description: string;
  readonly material: string;
  readonly fit: string;
  readonly care: string;
  readonly priceToman: number;
  readonly tag: "new" | "essential" | null;
  readonly provenance: ProductProvenance;
  readonly colorways: readonly [Colorway, ...Colorway[]];
};

const temporaryProducts: readonly Product[] = [
  {
    id: "sh-101",
    slug: "minimal-cotton-top",
    name: "تاپ نخی مینیمال",
    category: "تاپ",
    description:
      "همان انتخاب راحتی که صبح بی‌فکر سراغش می‌روی. برش آزاد و سرشانهٔ پهن، این تاپ را برای پوشیدن با جین یا زیر یک لایهٔ گرم مناسب می‌کند.",
    material: "۱۰۰٪ پنبه",
    fit: "آزاد، بدون آستین",
    care: "شست‌وشوی ملایم با آب سرد؛ خشک‌کردن در سایه.",
    priceToman: 1290000,
    tag: "new",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "white",
        label: "سفید",
        swatch: "#f0efea",
        imageNote: null,
        images: [
          {
            src: "/images/cotton-top.webp",
            alt: "تاپ سفید بدون آستین با سرشانهٔ پهن، همراه جین آبی",
            position: "center 35%",
            width: 800,
            height: 1200,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH101-WH-S", size: "S", availability: "available" },
          { sku: "SH101-WH-M", size: "M", availability: "available" },
          { sku: "SH101-WH-L", size: "L", availability: "sold-out" },
          { sku: "SH101-WH-XL", size: "XL", availability: "available" },
        ],
      },
    ],
  },
  {
    id: "sh-102",
    slug: "fringed-knit-poncho",
    name: "پانچوی بافت ریشه‌دار",
    category: "بافت",
    description:
      "بافتی با فرم رها برای روزهای خنک. ریشه‌های ظریف و رنگ روشنش، یک لایهٔ آرام به ترکیب‌های روزمره اضافه می‌کند.",
    material: "ترکیب پنبه و الیاف بافت",
    fit: "آزاد، یقهٔ هفت، لبهٔ ریشه‌دار",
    care: "با دست و آب سرد شسته شود؛ روی سطح صاف خشک شود.",
    priceToman: 2490000,
    tag: "essential",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "oat",
        label: "جو دوسر",
        swatch: "#d6c9b7",
        imageNote: null,
        images: [
          {
            src: "/images/knit-poncho.webp",
            alt: "پانچوی بافت روشن با یقهٔ هفت و لبه‌های ریشه‌دار روی چوب‌لباسی",
            position: "center 45%",
            width: 800,
            height: 1148,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH102-OT-S", size: "S", availability: "available" },
          { sku: "SH102-OT-M", size: "M", availability: "available" },
          { sku: "SH102-OT-L", size: "L", availability: "sold-out" },
        ],
      },
    ],
  },
  {
    id: "sh-103",
    slug: "patchwork-denim",
    name: "جین وصله‌دوزی‌شده",
    category: "شلوار",
    description:
      "جین آبی با وصله‌های رنگی و جزئیات زاپ‌دار؛ یک انتخاب پرشخصیت در کنار تاپ‌ها و رویه‌های ساده.",
    material: "۹۸٪ پنبه، ۲٪ الاستین",
    fit: "راسته، فاق متوسط، زاپ‌دار",
    care: "پشت‌ورو و با رنگ‌های مشابه شسته شود.",
    priceToman: 2790000,
    tag: null,
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "blue",
        label: "آبی دنیم",
        swatch: "#657c88",
        imageNote: null,
        images: [
          {
            src: "/images/denim.webp",
            alt: "جین آبی زاپ‌دار با وصله‌های رنگی و کفش کتانی سفید",
            position: "center 45%",
            width: 800,
            height: 1200,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH103-BL-26", size: "۲۶", availability: "sold-out" },
          { sku: "SH103-BL-28", size: "۲۸", availability: "available" },
          { sku: "SH103-BL-30", size: "۳۰", availability: "available" },
          { sku: "SH103-BL-32", size: "۳۲", availability: "available" },
        ],
      },
    ],
  },
  {
    id: "sh-104",
    slug: "rust-bomber-jacket",
    name: "کاپشن بامبر آجری",
    category: "رویه",
    description:
      "لایه‌ای سبک برای هوای بین فصل‌ها. رنگ گرم آجری و فرم کوتاه آن با جین و شلوارهای راحت روزمره کنار می‌آید.",
    material: "پارچهٔ کج‌راه با آستر سبک",
    fit: "کوتاه، لبه و سرآستین کش‌بافت",
    care: "شست‌وشوی ملایم در ۳۰ درجه؛ اتوکشی با حرارت کم.",
    priceToman: 3290000,
    tag: "new",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "rust",
        label: "آجری",
        swatch: "#ac6443",
        imageNote: null,
        images: [
          {
            src: "/images/rust-bomber.webp",
            alt: "کاپشن بامبر آجری با زیپ و سرآستین کش‌بافت روی چوب‌لباسی",
            position: "center 45%",
            width: 800,
            height: 1066,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH104-RU-S", size: "S", availability: "available" },
          { sku: "SH104-RU-M", size: "M", availability: "available" },
          { sku: "SH104-RU-L", size: "L", availability: "available" },
        ],
      },
    ],
  },
  {
    id: "sh-105",
    slug: "rose-cargo-trousers",
    name: "شلوار راحت صورتی",
    category: "شلوار",
    description:
      "فرمی رها، جیب‌های کاربردی و صورتی ملایم. برای روزهایی که راحتی را با کمی رنگ می‌خواهی؛ کنار یک تاپ ساده کامل می‌شود.",
    material: "ترکیب پنبه و الیاف مقاوم",
    fit: "راحت، جیب‌دار، دم‌پای جمع",
    care: "شست‌وشوی ملایم؛ اتو از پشت پارچه.",
    priceToman: 2190000,
    tag: "essential",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "rose",
        label: "صورتی چرک",
        swatch: "#bd8f91",
        imageNote: null,
        images: [
          {
            src: "/images/rose-cargo.webp",
            alt: "شلوار کارگوی صورتی ملایم با جیب‌های کناری و دم‌پای جمع",
            position: "center 40%",
            width: 800,
            height: 1200,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH105-RO-S", size: "S", availability: "available" },
          { sku: "SH105-RO-M", size: "M", availability: "sold-out" },
          { sku: "SH105-RO-L", size: "L", availability: "available" },
        ],
      },
    ],
  },
  {
    id: "sh-106",
    slug: "white-day-dress",
    name: "پیراهن کوتاه سفید",
    category: "پیراهن",
    description:
      "پیراهنی سبک با یقهٔ باز و جزئیات ظریف پارچه؛ برای روزهای روشن، سفرهای کوتاه و قرارهای بی‌تکلف.",
    material: "پنبهٔ گلدوزی‌شده با آستر سبک",
    fit: "کوتاه، کمر جمع، یقهٔ باز",
    care: "با آب سرد و شویندهٔ ملایم شسته شود.",
    priceToman: 2890000,
    tag: "new",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "warm-white",
        label: "سفید گرم",
        swatch: "#f1ede4",
        imageNote: null,
        images: [
          {
            src: "/images/everyday-dress.webp",
            alt: "پیراهن سفید کوتاه با یقهٔ باز و حاشیهٔ گلدوزی روی پله‌های یک ساختمان",
            position: "center 30%",
            width: 800,
            height: 1199,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH106-WW-XS", size: "XS", availability: "available" },
          { sku: "SH106-WW-S", size: "S", availability: "available" },
          { sku: "SH106-WW-M", size: "M", availability: "available" },
          { sku: "SH106-WW-L", size: "L", availability: "sold-out" },
        ],
      },
    ],
  },
  {
    id: "sh-107",
    slug: "violet-lace-dress",
    name: "پیراهن گیپور بنفش",
    category: "پیراهن",
    description:
      "نقش ظریف گیپور، آستین بلند و رنگ بنفش عمیق. انتخابی متفاوت برای موقعیت‌هایی که دلت جزئیات بیشتری می‌خواهد.",
    material: "گیپور با آستر سبک",
    fit: "جذب، آستین بلند",
    care: "خشک‌شویی ملایم؛ دور از زیورآلات تیز نگهداری شود.",
    priceToman: 3490000,
    tag: null,
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "violet",
        label: "بنفش تیره",
        swatch: "#3b2b67",
        imageNote: null,
        images: [
          {
            src: "/images/violet-lace.webp",
            alt: "پیراهن گیپور بنفش تیره با آستین بلند و نقش گل",
            position: "center 35%",
            width: 800,
            height: 1067,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH107-VI-S", size: "S", availability: "sold-out" },
          { sku: "SH107-VI-M", size: "M", availability: "sold-out" },
          { sku: "SH107-VI-L", size: "L", availability: "sold-out" },
        ],
      },
    ],
  },
  {
    id: "sh-108",
    slug: "city-coat",
    name: "پالتوی شهری",
    category: "رویه",
    description:
      "لایهٔ آخر یک ترکیب ساده؛ با فرم بلند و خطی تمیز. یک همراه شهری برای لباس‌هایی که از قبل دوستشان داری.",
    material: "ترکیب پشم و الیاف مقاوم",
    fit: "آزاد، یقهٔ برگردان، قد بلند",
    care: "خشک‌شویی ملایم؛ روی چوب‌لباسی پهن نگهداری شود.",
    priceToman: 4890000,
    tag: "new",
    provenance: {
      kind: "temporary",
      imageCredit: "Unsplash",
      note: "محصول، قیمت و مشخصات نمونه؛ عکس موقت از Unsplash و متعلق به موجودی شواندی نیست.",
    },
    colorways: [
      {
        id: "blue",
        label: "آبی غبارآلود",
        swatch: "#789198",
        imageNote: null,
        images: [
          {
            src: "/images/hero.webp",
            alt: "پالتوی بلند آبی در یک استایل خیابانی پاییزی",
            position: "center 35%",
            width: 1600,
            height: 2400,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH108-BL-S", size: "S", availability: "available" },
          { sku: "SH108-BL-M", size: "M", availability: "sold-out" },
          { sku: "SH108-BL-L", size: "L", availability: "available" },
        ],
      },
      {
        id: "navy",
        label: "سرمه‌ای",
        swatch: "#303d49",
        imageNote:
          "تصویر رنگ سرمه‌ای هنوز آماده نیست؛ تصویر مرجع رنگ آبی نمایش داده شده است.",
        images: [
          {
            src: "/images/hero.webp",
            alt: "تصویر مرجع پالتوی آبی؛ عکس رنگ سرمه‌ای آماده نیست",
            position: "center 35%",
            width: 1600,
            height: 2400,
            fit: "cover",
          },
        ],
        variants: [
          { sku: "SH108-NV-S", size: "S", availability: "sold-out" },
          { sku: "SH108-NV-M", size: "M", availability: "available" },
          { sku: "SH108-NV-L", size: "L", availability: "available" },
        ],
      },
    ],
  },
];

export const realProducts: readonly Product[] = [
  {
    id: "sh-real-76744",
    slug: "shark-wearable-blanket",
    name: "پتو کوسه‌ای؛ قد ۱۴۰ تا ۱۷۵",
    category: "پتو تنپوش",
    description:
      "تنپوش کوسه‌ای از شواندی استور. در عکس مرجع دو رنگ طوسی و آبی یخی کنار هم دیده می‌شوند. بازهٔ قد درج‌شده در منبع ۱۴۰ تا ۱۷۵ سانتی‌متر است.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "بازهٔ قد اعلام‌شده در منبع: ۱۴۰ تا ۱۷۵ سانتی‌متر.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 3480000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage: "https://shavandistore.ir/product/76744/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی استور",
        "برای قد ۱۴۰ تا ۱۷۵ سانتی‌متر",
        "رنگ‌های درج‌شده: طوسی، آبی یخی، صورتی، سرخابی",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "طوسی و آبی یخی در عکس",
        swatch: "#a0b4b7",
        imageNote:
          "عکس اصلی شواندی استور، شامل طوسی و آبی یخی؛ برای سایر رنگ‌های درج‌شده عکس جداگانه تأیید نشده است.",
        images: [
          {
            src: "/images/shavandi/shark-wearable-blanket-140-175.webp",
            alt: "دو تنپوش کوسه‌ای آبی یخی و طوسی با دهان سفید و دندان‌های کوسه، عکس شواندی استور",
            position: "center",
            width: 552,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/shark-wearable-blanket-140-175-320.webp 320w, /images/shavandi/shark-wearable-blanket-140-175-480.webp 480w, /images/shavandi/shark-wearable-blanket-140-175.webp 552w",
          },
        ],
        variants: [
          {
            sku: "SH-76744-REF-1",
            size: "قد ۱۴۰–۱۷۵",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-109677",
    slug: "panda-wearable-blanket",
    name: "پتو تنپوش مدل پاندا",
    category: "پتو تنپوش",
    description:
      "تنپوش طرح پاندا با ترکیب سیاه و سفید در عکس اصلی شواندی استور. اندازه، جنس و روش نگهداری در صفحهٔ منبع مشخص نشده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه و اندازه‌گیری در منبع مشخص نشده؛ انتخاب اندازه در این صفحه نمایشی است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 5380000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage: "https://shavandistore.ir/product/109677/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: ["نام، عکس و قیمت از شواندی استور"],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "انتخاب اندازه صرفاً نمونه است؛ اندازهٔ واقعی تأیید نشده.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "سیاه و سفید در عکس",
        swatch: "#323432",
        imageNote:
          "عکس اصلی محصول در شواندی استور؛ انتخاب اندازه و موجودی فقط برای پیش‌نمایش است.",
        images: [
          {
            src: "/images/shavandi/panda-wearable-blanket.webp",
            alt: "پتو تنپوش سیاه و سفید با کلاه صورت پاندا، عکس شواندی استور",
            position: "center",
            width: 518,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/panda-wearable-blanket-320.webp 320w, /images/shavandi/panda-wearable-blanket-480.webp 480w, /images/shavandi/panda-wearable-blanket.webp 518w",
          },
        ],
        variants: [
          {
            sku: "SH-109677-REF-1",
            size: "نمایشی",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-114173",
    slug: "long-ear-rabbit-wearable-blanket",
    name: "پتو تنپوش خرگوش گوش‌بلند",
    category: "پتو تنپوش",
    description:
      "تنپوش خرگوش با گوش‌های بلند. عکس اصلی شواندی استور رنگ‌های بنفش و سرخابی را کنار هم نشان می‌دهد؛ تصویر جداگانهٔ همهٔ رنگ‌های فهرست‌شده در اختیار این پیش‌نمایش نیست.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه و اندازه‌گیری در منبع مشخص نشده؛ انتخاب اندازه در این صفحه نمایشی است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 5380000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage: "https://shavandistore.ir/product/114173/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی استور",
        "رنگ‌های درج‌شده: سرخابی، طوسی، بنفش، قرمز",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "انتخاب اندازه صرفاً نمونه است؛ اندازهٔ واقعی تأیید نشده.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "بنفش و سرخابی در عکس",
        swatch: "#c48db9",
        imageNote:
          "عکس اصلی شواندی استور، شامل بنفش و سرخابی؛ رنگ‌های دیگر عکس جداگانهٔ تأییدشده ندارند.",
        images: [
          {
            src: "/images/shavandi/long-ear-rabbit-wearable-blanket.webp",
            alt: "دو تنپوش خرگوش گوش‌بلند بنفش و سرخابی کنار هم، عکس شواندی استور",
            position: "center",
            width: 554,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/long-ear-rabbit-wearable-blanket-320.webp 320w, /images/shavandi/long-ear-rabbit-wearable-blanket-480.webp 480w, /images/shavandi/long-ear-rabbit-wearable-blanket.webp 554w",
          },
        ],
        variants: [
          {
            sku: "SH-114173-REF-1",
            size: "نمایشی",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-106387",
    slug: "dinosaur-wearable-blanket",
    name: "تنپوش طرح دایناسور",
    category: "پتو تنپوش",
    description:
      "تنپوش طرح دایناسور از شواندی استور؛ عکس اصلی، کلاه دایناسور و ترکیب سبز لباس را نشان می‌دهد. اندازه، جنس و نگهداری در منبع مشخص نشده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه و اندازه‌گیری در منبع مشخص نشده؛ انتخاب اندازه در این صفحه نمایشی است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 5380000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage: "https://shavandistore.ir/product/106387/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: ["نام، عکس و قیمت از شواندی استور"],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "انتخاب اندازه صرفاً نمونه است؛ اندازهٔ واقعی تأیید نشده.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "سبز در عکس",
        swatch: "#69a677",
        imageNote:
          "عکس اصلی محصول در شواندی استور؛ انتخاب اندازه و موجودی فقط برای پیش‌نمایش است.",
        images: [
          {
            src: "/images/shavandi/dinosaur-wearable-blanket.webp",
            alt: "تنپوش سبز طرح دایناسور با کلاه و دندان‌های سفید، عکس شواندی استور",
            position: "center",
            width: 434,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/dinosaur-wearable-blanket-320.webp 320w, /images/shavandi/dinosaur-wearable-blanket.webp 434w",
          },
        ],
        variants: [
          {
            sku: "SH-106387-REF-1",
            size: "نمایشی",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-114687",
    slug: "real-madrid-white-shirt",
    name: "رئال مادرید سفید",
    category: "ورزشی",
    description:
      "پیراهن سفید رئال مادرید از شواندی اسپرت. نام، عکس، قیمت و اندازه‌های این صفحه از منبع ثبت شده‌اند. نسخهٔ هواداری و مجوز رسمی تأیید نشده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه‌های درج‌شده در منبع: S، M، L، XL و 2XL؛ جدول اندازه‌گیری ارائه نشده است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 1780000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage:
        "https://shavandisport.ir/product/114687/%D8%B1%D8%A6%D8%A7%D9%84-%D9%85%D8%A7%D8%AF%D8%B1%DB%8C%D8%AF-%D8%B3%D9%81%DB%8C%D8%AF/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی اسپرت",
        "اندازه‌های منبع: S، M، L، XL، 2XL",
        "رنگ سفید در نام منبع",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "سفید",
        swatch: "#efeeeb",
        imageNote:
          "عکس اصلی شواندی اسپرت. نسخهٔ هواداری، فصل و مجوز رسمی در منبع تأیید نشده‌اند.",
        images: [
          {
            src: "/images/shavandi/real-madrid-white.webp",
            alt: "پیراهن سفید رئال مادرید با نشان تیم و نوشتهٔ امارات، عکس شواندی اسپرت",
            position: "center",
            width: 600,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/real-madrid-white-320.webp 320w, /images/shavandi/real-madrid-white-480.webp 480w, /images/shavandi/real-madrid-white.webp 600w",
          },
        ],
        variants: [
          {
            sku: "SH-114687-REF-1",
            size: "S",
            availability: "available",
          },
          {
            sku: "SH-114687-REF-2",
            size: "M",
            availability: "available",
          },
          {
            sku: "SH-114687-REF-3",
            size: "L",
            availability: "available",
          },
          {
            sku: "SH-114687-REF-4",
            size: "XL",
            availability: "available",
          },
          {
            sku: "SH-114687-REF-5",
            size: "2XL",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-129615",
    slug: "france-third-player-world-cup-kit",
    name: "کیت جام جهانی پلیری سوم فرانسه",
    category: "ورزشی",
    description:
      "کیت سوم جام جهانی فرانسه از شواندی اسپرت. عبارت «پلیری» در نام منبع آمده است؛ فصل، جنس، مجوز رسمی و نسخهٔ هواداری تأیید نشده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه‌های درج‌شده در منبع: اسمال تا دوایکس لارج؛ جدول اندازه‌گیری ارائه نشده است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 2780000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage:
        "https://shavandisport.ir/product/129615/%DA%A9%DB%8C%D8%AA-%D8%AC%D8%A7%D9%85-%D8%AC%D9%87%D8%A7%D9%86%DB%8C-%D9%BE%D9%84%DB%8C%D8%B1%DB%8C-%D8%B3%D9%88%D9%85-%D9%81%D8%B1%D8%A7%D9%86%D8%B3%D9%87/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی اسپرت",
        "نسخهٔ پلیری و کیت سوم جام جهانی در نام منبع",
        "اندازه‌های منبع: اسمال، مدیوم، لارج، ایکس لارج، دوایکس لارج",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "رنگ عکس مرجع",
        swatch: "#b5c9c5",
        imageNote:
          "عکس اصلی شواندی اسپرت؛ عنوان «پلیری» از نام منبع است و به معنای تأیید مجوز رسمی نیست.",
        images: [
          {
            src: "/images/shavandi/france-third-player-world-cup.webp",
            alt: "پیراهن روشن فرانسه با نشان خروس و لبهٔ آستین آبی سفید قرمز، عکس شواندی اسپرت",
            position: "center",
            width: 600,
            height: 540,
            fit: "contain",
            srcSet:
              "/images/shavandi/france-third-player-world-cup-320.webp 320w, /images/shavandi/france-third-player-world-cup-480.webp 480w, /images/shavandi/france-third-player-world-cup.webp 600w",
          },
        ],
        variants: [
          {
            sku: "SH-129615-REF-1",
            size: "S",
            availability: "available",
          },
          {
            sku: "SH-129615-REF-2",
            size: "M",
            availability: "available",
          },
          {
            sku: "SH-129615-REF-3",
            size: "L",
            availability: "available",
          },
          {
            sku: "SH-129615-REF-4",
            size: "XL",
            availability: "available",
          },
          {
            sku: "SH-129615-REF-5",
            size: "2XL",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-130179",
    slug: "portugal-black-shirt-shorts-set",
    name: "ست تیشرت شلوارک پرتغال مشکی",
    category: "ورزشی",
    description:
      "ست تیشرت و شلوارک مشکی پرتغال از شواندی اسپرت. هر دو بخش ست در عکس اصلی دیده می‌شوند؛ جنس، فصل و نوع نسخه در منبع مشخص نشده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه‌های درج‌شده در منبع: اسمال تا دوایکس لارج؛ جدول اندازه‌گیری ارائه نشده است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 1680000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage:
        "https://shavandisport.ir/product/130179/%D8%B3%D8%AA-%D8%AA%DB%8C%D8%B4%D8%B1%D8%AA-%D8%B4%D9%84%D9%88%D8%A7%D8%B1%DA%A9-%D9%BE%D8%B1%D8%AA%D8%BA%D8%A7%D9%84-%D9%85%D8%B4%DA%A9%DB%8C/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی اسپرت",
        "ست تیشرت و شلوارک؛ مشکی در نام منبع",
        "اندازه‌های منبع: اسمال، مدیوم، لارج، ایکس لارج، دوایکس لارج",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "مشکی",
        swatch: "#262626",
        imageNote:
          "عکس اصلی ست از شواندی اسپرت؛ نسخهٔ هواداری، فصل و مجوز رسمی تأیید نشده‌اند.",
        images: [
          {
            src: "/images/shavandi/portugal-black-shirt-shorts-set.webp",
            alt: "ست پیراهن و شلوارک مشکی پرتغال با نشان‌های طلایی، عکس شواندی اسپرت",
            position: "center",
            width: 480,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/portugal-black-shirt-shorts-set-320.webp 320w, /images/shavandi/portugal-black-shirt-shorts-set.webp 480w",
          },
        ],
        variants: [
          {
            sku: "SH-130179-REF-1",
            size: "S",
            availability: "available",
          },
          {
            sku: "SH-130179-REF-2",
            size: "M",
            availability: "available",
          },
          {
            sku: "SH-130179-REF-3",
            size: "L",
            availability: "available",
          },
          {
            sku: "SH-130179-REF-4",
            size: "XL",
            availability: "available",
          },
          {
            sku: "SH-130179-REF-5",
            size: "2XL",
            availability: "available",
          },
        ],
      },
    ],
  },
  {
    id: "sh-real-128626",
    slug: "argentina-tshirt",
    name: "تیشرت آرژانتین",
    category: "ورزشی",
    description:
      "تیشرت آرژانتین از شواندی اسپرت؛ عکس مرجع یک تیشرت آبی روشن با شمارهٔ ۱۰ را نشان می‌دهد. اندازه‌های L، XL و 2XL در توضیح منبع آمده‌اند.",
    material: "در صفحهٔ منبع مشخص نشده است.",
    fit: "اندازه‌های درج‌شده در منبع: L، XL و 2XL؛ جدول اندازه‌گیری ارائه نشده است.",
    care: "در صفحهٔ منبع مشخص نشده است؛ اطلاعات این پیش‌نمایش راهنمای نگهداری محصول نیست.",
    priceToman: 1480000,
    tag: null,
    provenance: {
      kind: "sourced",
      sourcePage:
        "https://shavandisport.ir/product/128626/%D8%AA%DB%8C%D8%B4%D8%B1%D8%AA-%D8%A7%D8%B1%DA%98%D8%A7%D9%86%D8%AA%DB%8C%D9%86/",
      checkedAt: "2026-11-02",
      rights: "confirmed-by-user",
      verifiedFacts: [
        "نام، عکس و قیمت از شواندی اسپرت",
        "اندازه‌های منبع: L، XL، 2XL",
      ],
      demoFacts: [
        "موجودی، سبد و تسویه نمایشی‌اند.",
        "جنس و روش نگهداری در منبع مشخص نشده‌اند.",
      ],
    },
    colorways: [
      {
        id: "source-photo",
        label: "رنگ عکس مرجع",
        swatch: "#b5c8cf",
        imageNote:
          "عکس اصلی شواندی اسپرت؛ نسخهٔ هواداری، جنس، فصل و مجوز رسمی تأیید نشده‌اند.",
        images: [
          {
            src: "/images/shavandi/argentina-shirt.webp",
            alt: "تیشرت آبی روشن آرژانتین با شمارهٔ ۱۰ روی چوب‌لباسی، عکس شواندی اسپرت",
            position: "center",
            width: 450,
            height: 600,
            fit: "contain",
            srcSet:
              "/images/shavandi/argentina-shirt-320.webp 320w, /images/shavandi/argentina-shirt.webp 450w",
          },
        ],
        variants: [
          {
            sku: "SH-128626-REF-1",
            size: "L",
            availability: "available",
          },
          {
            sku: "SH-128626-REF-2",
            size: "XL",
            availability: "available",
          },
          {
            sku: "SH-128626-REF-3",
            size: "2XL",
            availability: "available",
          },
        ],
      },
    ],
  },
];

export const products: readonly Product[] = [
  ...temporaryProducts,
  ...realProducts,
];

export type Collection = {
  readonly slug: string;
  readonly name: string;
  readonly eyebrow: string;
  readonly description: string;
  readonly image: ProductImage;
  readonly productIds: readonly string[];
  readonly note: string;
};

export const collections: readonly Collection[] = [
  {
    slug: "wearable-blankets",
    name: "پتوهای تنپوش",
    eyebrow: "کوسه، پاندا، خرگوش و دایناسور",
    description:
      "عکس، نام و قیمت از شواندی استور؛ موجودی و تسویه نمایشی‌اند. تنها بازهٔ قد کوسه در منبع مشخص شده است.",
    image: {
      src: "/images/shavandi/shark-wearable-blanket-140-175.webp",
      alt: "دو تنپوش کوسه‌ای آبی یخی و طوسی با دهان سفید و دندان‌های کوسه، عکس شواندی استور",
      position: "center",
      width: 552,
      height: 600,
      fit: "contain",
      srcSet:
        "/images/shavandi/shark-wearable-blanket-140-175-320.webp 320w, /images/shavandi/shark-wearable-blanket-140-175-480.webp 480w, /images/shavandi/shark-wearable-blanket-140-175.webp 552w",
    },
    productIds: [
      "sh-real-76744",
      "sh-real-109677",
      "sh-real-114173",
      "sh-real-106387",
    ],
    note: "عکس و قیمت از منبع · موجودی نمایشی",
  },
  {
    slug: "football",
    name: "فوتبال و ورزش",
    eyebrow: "از شواندی اسپرت",
    description:
      "پیراهن‌ها و ست فوتبال با عکس و قیمت ثبت‌شده از شواندی اسپرت. نسخهٔ هواداری تأیید نشده؛ فقط عنوان پلیری فرانسه در منبع آمده است.",
    image: {
      src: "/images/shavandi/real-madrid-white.webp",
      alt: "پیراهن سفید رئال مادرید با نشان تیم و نوشتهٔ امارات، عکس شواندی اسپرت",
      position: "center",
      width: 600,
      height: 600,
      fit: "contain",
      srcSet:
        "/images/shavandi/real-madrid-white-320.webp 320w, /images/shavandi/real-madrid-white-480.webp 480w, /images/shavandi/real-madrid-white.webp 600w",
    },
    productIds: [
      "sh-real-114687",
      "sh-real-129615",
      "sh-real-130179",
      "sh-real-128626",
    ],
    note: "نسخهٔ هواداری تأیید نشده · موجودی نمایشی",
  },
  {
    slug: "men",
    name: "پوشاک مردانه",
    eyebrow: "این بخش در حال آماده‌شدن است",
    description:
      "هنوز محصول مردانهٔ تأییدشده‌ای برای این بخش اضافه نشده است. تصویر این دسته یک تصویرسازی موقت است و کالای قابل سفارش را نشان نمی‌دهد.",
    image: {
      src: "/images/mens-placeholder.svg",
      alt: "تصویرسازی موقت تی‌شرت مردانه؛ عکس محصول واقعی نیست",
      position: "center",
      width: 640,
      height: 720,
      fit: "contain",
    },
    productIds: [],
    note: "تصویر موقت · محصولی ثبت نشده",
  },
  {
    slug: "women",
    name: "پوشاک زنانه",
    eyebrow: "چند انتخاب برای پیش‌نمایش",
    description:
      "عکس‌های موقت Unsplash برای نمایش پوشاک زنانه؛ این محصولات، قیمت‌ها و مشخصات نمونه‌اند و موجودی واقعی شواندی را نشان نمی‌دهند.",
    image: {
      src: "/images/cotton-top.webp",
      alt: "عکس موقت یک تاپ سفید زنانه از Unsplash",
      position: "center 35%",
      width: 800,
      height: 1200,
      fit: "cover",
    },
    productIds: [
      "sh-101",
      "sh-102",
      "sh-103",
      "sh-104",
      "sh-105",
      "sh-106",
      "sh-107",
      "sh-108",
    ],
    note: "عکس و محصول موقت · همهٔ اطلاعات نمونه",
  },
  {
    slug: "everyday",
    name: "پایه‌های هر روز",
    eyebrow: "مجموعهٔ نمونه",
    description:
      "تاپ و شلوار در کاتالوگ نمونه با عکس‌های موقت Unsplash؛ این محصولات متعلق به موجودی واقعی شواندی نیستند.",
    image: {
      src: "/images/wardrobe.webp",
      alt: "عکس موقت لباس‌های روشن روی رگال از Unsplash",
      position: "center",
      width: 1000,
      height: 667,
      fit: "cover",
    },
    productIds: ["sh-101", "sh-103", "sh-105"],
    note: "عکس و اطلاعات موقت",
  },
  {
    slug: "soft-layers",
    name: "لایه‌های نرم",
    eyebrow: "مجموعهٔ نمونه",
    description:
      "بافت و رویه‌های کاتالوگ نمونه با عکس‌های موقت Unsplash؛ قیمت، اندازه و موجودی نمونه‌اند.",
    image: {
      src: "/images/knit-poncho.webp",
      alt: "عکس موقت پانچوی بافت روشن از Unsplash",
      position: "center",
      width: 800,
      height: 1148,
      fit: "cover",
    },
    productIds: ["sh-102", "sh-104", "sh-108"],
    note: "عکس و اطلاعات موقت",
  },
  {
    slug: "out-in-the-city",
    name: "قرارهای شهری",
    eyebrow: "مجموعهٔ نمونه",
    description:
      "چند انتخاب از کاتالوگ نمونه برای نمایش صفحهٔ مجموعه؛ عکس‌ها موقت‌اند و موجودی واقعی شواندی را نشان نمی‌دهند.",
    image: {
      src: "/images/collection.webp",
      alt: "عکس موقت استایل خیابانی از Unsplash",
      position: "center",
      width: 1000,
      height: 667,
      fit: "cover",
    },
    productIds: ["sh-103", "sh-106", "sh-107", "sh-108"],
    note: "عکس و اطلاعات موقت",
  },
];

export const homeCollections = collections.slice(0, 4);

export function provenanceLabel(provenance: ProductProvenance): string {
  switch (provenance.kind) {
    case "sourced":
      return "عکس و اطلاعات از شواندی";
    case "temporary":
      return "تصویر و محصول موقت";
    case "local-edit":
      return "ویرایش محلی · اطلاعات تأیید نشده";
  }
}

export function priceLabel(provenance: ProductProvenance): string {
  switch (provenance.kind) {
    case "sourced":
      return "قیمت منبع · ۱۱ آبان ۱۴۰۵";
    case "temporary":
      return "قیمت نمونه";
    case "local-edit":
      return "قیمت در ویرایش محلی";
  }
}

export function isAvailable(product: Product): boolean {
  return product.colorways.some((colorway) =>
    colorway.variants.some((variant) => variant.availability === "available"),
  );
}

export function availableSizes(product: Product): readonly Size[] {
  const sizeOrder: readonly Size[] = [
    "XS",
    "S",
    "M",
    "L",
    "XL",
    "2XL",
    "قد ۱۴۰–۱۷۵",
    "نمایشی",
    "۲۶",
    "۲۸",
    "۳۰",
    "۳۲",
  ];
  const available = new Set(
    product.colorways.flatMap((colorway) =>
      colorway.variants
        .filter((variant) => variant.availability === "available")
        .map((variant) => variant.size),
    ),
  );
  return sizeOrder.filter((size) => available.has(size));
}

const toman = new Intl.NumberFormat("fa-IR");

export function formatPrice(priceToman: number): string {
  return toman.format(priceToman);
}
