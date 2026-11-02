export type Size = "XS" | "S" | "M" | "L" | "XL" | "۲۶" | "۲۸" | "۳۰" | "۳۲";

export type Variant = {
  readonly sku: string;
  readonly size: Size;
  readonly availability: "available" | "sold-out";
};

export type ProductImage = {
  readonly src: string;
  readonly alt: string;
  readonly position: string;
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
  readonly category: "تاپ" | "بافت" | "شلوار" | "رویه" | "پیراهن";
  readonly description: string;
  readonly material: string;
  readonly fit: string;
  readonly care: string;
  readonly priceToman: number;
  readonly tag: "new" | "essential" | null;
  readonly colorways: readonly [Colorway, ...Colorway[]];
};

export const products: readonly Product[] = [
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

export const collections = [
  {
    slug: "everyday",
    name: "پایه‌های هر روز",
    eyebrow: "ساده، برای بارها پوشیدن",
    description:
      "تاپ‌های راحت و شلوارهایی که ترکیب‌کردنشان آسان است. نقطهٔ شروع یک کمد با انتخاب‌های فکرشده.",
    image: "/images/wardrobe.webp",
    imageAlt: "لباس‌های روشن و خنثی روی رگال چوبی",
    productIds: ["sh-101", "sh-103", "sh-105"],
  },
  {
    slug: "soft-layers",
    name: "لایه‌های نرم",
    eyebrow: "برای هوای خنک",
    description:
      "بافت و رویه‌هایی برای کنار هم پوشیدن؛ لایه‌های سبک با بافت‌هایی که به روزهای سرد گرما می‌دهند.",
    image: "/images/knit-poncho.webp",
    imageAlt: "پانچوی بافت روشن برای لایه‌پوشی پاییزی",
    productIds: ["sh-102", "sh-104", "sh-108"],
  },
  {
    slug: "out-in-the-city",
    name: "قرارهای شهری",
    eyebrow: "از صبح تا عصر",
    description:
      "چند انتخاب برای بیرون‌رفتن‌های روزانه؛ لباس‌هایی با جزئیات متفاوت و حال‌وهوایی آرام.",
    image: "/images/collection.webp",
    imageAlt: "استایل خیابانی با پالتوی زرشکی و کیف‌های خرید",
    productIds: ["sh-103", "sh-106", "sh-107", "sh-108"],
  },
] satisfies readonly {
  slug: string;
  name: string;
  eyebrow: string;
  description: string;
  image: string;
  imageAlt: string;
  productIds: readonly string[];
}[];

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
