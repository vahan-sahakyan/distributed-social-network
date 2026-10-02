import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { TrendingUp } from 'lucide-react'
import { api } from '../api'

// TrendingHashtags lists the most used hashtags of the last 24 hours.
export function TrendingHashtags() {
  const [tags, setTags] = useState(null)

  useEffect(() => {
    const load = () =>
      api.trendingHashtags(24, 8)
        .then(d => setTags(d.hashtags || []))
        .catch(() => setTags([]))
    load()
    const timer = setInterval(load, 60_000)
    return () => clearInterval(timer)
  }, [])

  if (!tags || tags.length === 0) return null

  return (
    <div className="bg-surface border border-border rounded-xl p-4">
      <p className="flex items-center gap-1.5 text-[10px] font-semibold text-muted uppercase tracking-widest mb-3">
        <TrendingUp size={11} /> Trending
      </p>
      <div className="space-y-2">
        {tags.map(t => (
          <Link
            key={t.hashtag}
            to={`/search?q=${encodeURIComponent('#' + t.hashtag)}`}
            className="flex items-center justify-between text-xs group"
          >
            <span className="text-text group-hover:text-accent transition-colors">#{t.hashtag}</span>
            <span className="text-muted">{t.posts} {t.posts === 1 ? 'post' : 'posts'}</span>
          </Link>
        ))}
      </div>
    </div>
  )
}
