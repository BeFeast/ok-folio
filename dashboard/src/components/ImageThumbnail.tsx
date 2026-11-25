import { getPhotoThumbnailUrl } from '../api';

interface ImageThumbnailProps {
  photoId: number;
  title: string;
  onClick?: () => void;
  className?: string;
}

export default function ImageThumbnail({ photoId, title, onClick, className = '' }: ImageThumbnailProps) {
  return (
    <div
      className={`overflow-hidden bg-[color:var(--folio-surface-muted)] cursor-pointer transition-opacity hover:opacity-85 ${className || 'w-32 h-24'}`}
      onClick={onClick}
      title={title}
    >
      <img
        src={getPhotoThumbnailUrl(photoId)}
        alt={title}
        className="w-full h-full object-cover"
        loading="lazy"
      />
    </div>
  );
}
