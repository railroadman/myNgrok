// Covers the inline-SVG icon lookup: a known `name` renders its path data, an unknown name
// renders an empty path instead of throwing, and the `size` prop controls width/height with
// a sensible 18px default when omitted.
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Icon from './Icon.vue'

describe('Icon', () => {
  it('renders the path data for a known icon name', () => {
    const wrapper = mount(Icon, { props: { name: 'tokens' } })
    expect(wrapper.find('path').attributes('d')).toContain('M8 12a4 4 0 1 1 3.46-2')
  })

  it('renders an empty path for an unknown icon name instead of throwing', () => {
    const wrapper = mount(Icon, { props: { name: 'does-not-exist' } })
    expect(wrapper.find('path').attributes('d')).toBe('')
  })

  it('defaults to size 18 and honors an explicit size prop', () => {
    const withDefault = mount(Icon, { props: { name: 'agents' } })
    expect(withDefault.find('svg').attributes('width')).toBe('18')
    expect(withDefault.find('svg').attributes('height')).toBe('18')

    const withSize = mount(Icon, { props: { name: 'agents', size: 24 } })
    expect(withSize.find('svg').attributes('width')).toBe('24')
  })
})
