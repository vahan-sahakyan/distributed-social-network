import { useState, useRef, useEffect } from 'react'
import { Image, Send, X } from 'lucide-react'
import { Avatar } from './Avatar'
import { api } from '../api'
import { useStore } from '../store'

export function PostComposer({ onPost }) {
  const { currentUser, toast } = useStore()
  const [text, setText] = useState('')
  const [uploading, setUploading] = useState(false)
  const [imageId, setImageId] = useState(null)
  const [imagePreview, setImagePreview] = useState(null)
  const [posting, setPosting] = useState(false)
  const fileRef = useRef()

  if (!currentUser) return null

  async function handleImage(e) {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      const res = await api.uploadMedia(file)
      setImageId(res.id)
      setImagePreview(URL.createObjectURL(file))
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setUploading(false)
      e.target.value = ''
    }
  }

  async function handlePost() {
    if (!text.trim() || posting) return
    setPosting(true)
    try {
      const post = await api.createPost(currentUser.id, text.trim(), imageId)
      setText('')
      setImageId(null)
      if (imagePreview) {
        URL.revokeObjectURL(imagePreview)
        setImagePreview(null)
      }
      toast('Post published!')
      onPost?.(post)
    } catch (err) {
      toast(err.message, 'error')
    } finally {
      setPosting(false)
    }
  }

  const overLimit = text.length > 280

  return (
    <div className="border-b border-border p-4 flex gap-3">
      <Avatar username={currentUser.username} size="sm" />
      <div className="flex-1 min-w-0">
        <textarea
          value={text}
          onChange={e => setText(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) handlePost() }}
          placeholder={`What's on your mind, @${currentUser.username}?`}
          rows={3}
          className="w-full bg-transparent text-sm text-text placeholder-muted resize-none outline-none leading-relaxed"
        />

        {imagePreview && (
          <div className="relative inline-block mt-2">
            <img src={imagePreview} alt="" className="max-h-48 rounded-xl object-cover border border-border" />
            <button
              onClick={() => { setImageId(null); URL.revokeObjectURL(imagePreview); setImagePreview(null) }}
              className="absolute top-1.5 right-1.5 bg-black/70 hover:bg-black/90 rounded-full p-0.5 text-white transition-colors"
            >
              <X size={12} />
            </button>
          </div>
        )}

        <div className="flex items-center justify-between mt-2 pt-2 border-t border-border/50">
          <button
            onClick={() => fileRef.current?.click()}
            disabled={uploading}
            title="Attach image"
            className="text-muted hover:text-accent transition-colors disabled:opacity-50"
          >
            <Image size={15} />
          </button>
          <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleImage} />

          <div className="flex items-center gap-3">
            <span className={`text-xs tabular-nums ${overLimit ? 'text-red-400' : 'text-muted/60'}`}>
              {text.length}/280
            </span>
            <button
              onClick={handlePost}
              disabled={!text.trim() || posting || overLimit}
              className="flex items-center gap-1.5 text-xs font-semibold bg-accent hover:bg-accent-hover disabled:opacity-40 text-white px-4 py-1.5 rounded-full transition-colors"
            >
              <Send size={11} />
              {posting ? 'Posting…' : 'Post'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
