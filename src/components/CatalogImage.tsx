import type { ProductImage } from "@/data/catalog";

export default function CatalogImage({
  image,
  sizes,
  eager = false,
}: {
  image: ProductImage;
  sizes: string;
  eager?: boolean;
}) {
  return (
    <img
      src={image.src}
      srcSet={image.srcSet}
      sizes={image.srcSet ? sizes : undefined}
      alt={image.alt}
      width={image.width}
      height={image.height}
      loading={eager ? "eager" : "lazy"}
      fetchPriority={eager ? "high" : undefined}
      decoding="async"
      style={{ objectPosition: image.position, objectFit: image.fit }}
    />
  );
}
