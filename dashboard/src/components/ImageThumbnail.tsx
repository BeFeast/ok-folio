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
      className={`bg-gray-200 rounded overflow-hidden cursor-pointer hover:opacity-80 transition-opacity ${className || 'w-32 h-24'}`}
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
