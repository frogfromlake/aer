import { describe, it, expect } from 'vitest';
import { humanizeMachineName, splitSubjectAndModel } from '../../src/lib/labels-core';

describe('humanizeMachineName', () => {
  it('title-cases snake_case names', () => {
    expect(humanizeMachineName('word_count')).toBe('Word Count');
    expect(humanizeMachineName('article_type')).toBe('Article Type');
    expect(humanizeMachineName('comment_count')).toBe('Comment Count');
  });

  it('preserves known acronyms / model names', () => {
    expect(humanizeMachineName('sentiment_score_bert_multilingual')).toBe(
      'Sentiment Score BERT Multilingual'
    );
    expect(humanizeMachineName('sentiment_score_sentiws')).toBe('Sentiment Score SentiWS');
    expect(humanizeMachineName('sentiment_score_bert_de_news')).toBe(
      'Sentiment Score BERT DE News'
    );
    expect(humanizeMachineName('image_url')).toBe('Image URL');
  });

  it('handles empty / single-token input', () => {
    expect(humanizeMachineName('')).toBe('');
    expect(humanizeMachineName('author')).toBe('Author');
  });
});

describe('splitSubjectAndModel', () => {
  it('splits the sentiment family on the " · " model separator', () => {
    expect(splitSubjectAndModel('Sentiment Score · BERT Multilingual')).toEqual({
      subject: 'Sentiment Score',
      model: 'BERT Multilingual'
    });
    expect(splitSubjectAndModel('Sentiment Score · SentiWS')).toEqual({
      subject: 'Sentiment Score',
      model: 'SentiWS'
    });
  });

  it('returns the whole label as subject when there is no model separator', () => {
    expect(splitSubjectAndModel('Word Count')).toEqual({ subject: 'Word Count', model: null });
    expect(splitSubjectAndModel('Entity Count')).toEqual({ subject: 'Entity Count', model: null });
  });
});
