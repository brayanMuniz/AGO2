export interface Post {
  id: number;
  rating: string;
  source: string;
  image_height: number;
  image_width: number;
  file_size: number;
  tags_artist: string[];
  tags_character: string[];
  tags_copyright: string[];
  tags_general: string[];
  tags_meta: string[];
  tag_count?: number;
  tag_count_artist?: number;
  tag_count_character?: number;
  tag_count_copyright?: number;
  tag_count_general?: number;
  tag_count_meta?: number;
  original_post_id?: string;
  original_source?: string;
}

export interface ImageData {
  id: number;
  file_name: string;
  hash: string;
  is_favorite: boolean;
  organized: boolean;
  main_data: Post | null;
  thumbnail_path: string;
  has_duplicate?: number | null;
  image_width: number;
  image_height: number;
  file_size: number;
}
